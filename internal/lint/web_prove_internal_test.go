package lint

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// webToolTimeout bounds one linter run on a fixture tree; the start of a
// typed ESLint run, with its TypeScript program, dominates it.
const webToolTimeout = 2 * time.Minute

// proveWebSet is the set-aware counterpart of proveManifest: every entry's
// red tree yields its code under the named set and its near tree does not.
// The manifest and the trees decode before any skip; a tool that resolves to
// no usable path skips the entry with the reason the acceptance lane records.
func proveWebSet(t *testing.T, manifest, set string, drivers []Driver) {
	t.Helper()
	m, err := proof.LoadManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Dir(manifest)
	for _, e := range m.Checks {
		tool, _ := webSplitID(t, e.ID)
		i := slices.IndexFunc(drivers, func(d Driver) bool { return d.Tool == tool })
		if i < 0 {
			t.Fatalf("%s names the tool %s, which no web driver runs", e.ID, tool)
		}
		d := drivers[i]
		red := webFixture(t, filepath.Join(base, e.Red))
		near := webFixture(t, filepath.Join(base, e.Near))
		t.Run(e.ID, func(t *testing.T) {
			t.Parallel()
			path := webToolPath(t, d.Tool)
			dir := webMaterialize(t, d, set)
			if n := webCount(webIssues(t, d, path, dir, set, red), e.Code); n == 0 {
				t.Errorf("the red tree of %s yields no %s under %s", e.ID, e.Code, set)
			}
			if n := webCount(webIssues(t, d, path, dir, set, near), e.Code); n != 0 {
				t.Errorf("the near tree of %s yields %s %d times under %s", e.ID, e.Code, n, set)
			}
		})
	}
}

// webManifestBound proves, with no tool, that every entry's id is
// lint.<tool>-<code>, that its code is the code the id names and that both
// trees decode to at least one file.
func webManifestBound(t *testing.T, manifest string) {
	t.Helper()
	m, err := proof.LoadManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Checks) == 0 {
		t.Fatalf("%s holds no entry", manifest)
	}
	base := filepath.Dir(manifest)
	for _, e := range m.Checks {
		if _, code := webSplitID(t, e.ID); code != e.Code {
			t.Errorf("%s carries the code %s, its id names %s", e.ID, e.Code, code)
		}
		webFixture(t, filepath.Join(base, e.Red))
		webFixture(t, filepath.Join(base, e.Near))
	}
}

// webSplitID answers the tool and the finding code a lint proof id names.
func webSplitID(t *testing.T, id string) (string, string) {
	t.Helper()
	tool, code, ok := strings.Cut(strings.TrimPrefix(id, "lint."), "-")
	if !ok || !strings.HasPrefix(id, "lint.") {
		t.Fatalf("%s is not lint.<tool>-<code>", id)
	}
	return tool, code
}

// webFixture reads a fixture tree at its in-tree paths, decoding .esc files.
func webFixture(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	tree := os.DirFS(dir)
	var names []string
	err := fs.WalkDir(tree, ".", func(name string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			names = append(names, name)
		}
		return err
	})
	if err != nil || len(names) == 0 {
		t.Fatalf("fixture %s holds no file: %v", dir, err)
	}
	files := make(map[string][]byte, len(names))
	for _, name := range names {
		data, err := fs.ReadFile(tree, name)
		if err != nil {
			t.Fatal(err)
		}
		if plain, ok := strings.CutSuffix(name, proof.EscapedSuffix); ok {
			name, data = plain, proof.Unescape(data)
		}
		files[name] = data
	}
	return files
}

// webToolPath resolves a pinned tool or skips with the stated reason.
func webToolPath(t *testing.T, tool string) string {
	t.Helper()
	pinned, ok := toolchain.Lookup(tool)
	if !ok {
		t.Fatalf("toolchain.yml pins no %s", tool)
	}
	resolved := testResolver(t).Full(t.Context(), pinned)
	if !resolved.Usable() {
		t.Skipf("%s %s not installed: run golaw setup --surface", pinned.Name, pinned.Version)
	}
	return resolved.Path
}

// webMaterialize writes one set of the driver's tree under a temporary tool
// directory, so a test never writes into the developer's tool directory.
// Materialize takes the set out of the tree itself.
func webMaterialize(t *testing.T, d Driver, set string) string {
	t.Helper()
	dir, err := toolchain.Materialize(t.TempDir(), "test", set, d.Config)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// webIssues writes a fixture tree as a repository root and runs the driver
// over the files of its languages, with the root as the working directory.
func webIssues(t *testing.T, d Driver, path, configDir, set string, files map[string][]byte) []Issue {
	t.Helper()
	// The tools report real paths, and a temporary directory behind a link
	// (/var on macOS) would leave every issue path absolute; golaw lint gets
	// its root from git, which resolves the link the same way.
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var read []string
	for name, data := range files {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err = os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(target, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if language, ok := surface.ByName(name); ok && slices.Contains(d.Languages, language) {
			read = append(read, name)
		}
	}
	slices.Sort(read)
	if len(read) == 0 {
		t.Fatalf("the fixture holds no file %s reads", d.Tool)
	}
	in := Input{Root: root, Files: read, ConfigDir: configDir, Set: set}
	req := toolchain.Request{Path: path, Args: d.Args(in), Dir: root, Env: toolchain.ToolEnv(testResolver(t).Env), Timeout: webToolTimeout}
	run, err := toolchain.Exec(t.Context(), req)
	if err != nil || run.Killed {
		t.Fatalf("%s did not finish: %v (killed %v)", d.Tool, err, run.Killed)
	}
	issues, err := Parse(d.Format, d.Tool, run.Output, root)
	if err != nil {
		t.Fatalf("%s output unreadable: %v\n%s", d.Tool, err, run.Output.Stderr)
	}
	return issues
}

// webCount counts the issues whose rule carries the finding code.
func webCount(issues []Issue, code string) int {
	n := 0
	for _, i := range issues {
		if Code(i.Rule) == code {
			n++
		}
	}
	return n
}
