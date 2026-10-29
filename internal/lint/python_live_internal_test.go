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
	"github.com/cristianargotti/golaw/internal/toolchain"
)

const (
	// pythonLintTimeout bounds one live run; pyright's first start on a
	// cold machine takes seconds, never a minute.
	pythonLintTimeout = time.Minute
	pythonDirMode     = 0o750
	pythonFileMode    = 0o600
	pythonTestKit     = "test"
)

// runPythonDriver runs one driver at its pinned tool over files of root
// under the set and answers the parsed issues; a tool that resolves to no
// usable path skips with the reason the acceptance lane records.
func runPythonDriver(t *testing.T, d Driver, set, root string, files []string) []Issue {
	t.Helper()
	tool, ok := toolchain.Lookup(d.Tool)
	if !ok {
		t.Fatalf("toolchain.yml holds no %s", d.Tool)
	}
	resolved := testResolver(t).Full(t.Context(), tool)
	if !resolved.Usable() {
		t.Skipf("%s %s not installed: run golaw setup --surface", tool.Name, tool.Version)
	}
	args := d.Args(Input{Root: root, Files: files, ConfigDir: pythonMaterialize(t, d, set), Set: set})
	if args == nil {
		return nil
	}
	req := toolchain.Request{Path: resolved.Path, Args: args, Dir: root, Env: toolchain.ToolEnv(os.Environ()), Timeout: pythonLintTimeout}
	ex, err := toolchain.Exec(t.Context(), req)
	if err != nil || ex.Killed {
		t.Fatalf("run %s %q: killed %v, %v", d.Tool, args, ex.Killed, err)
	}
	issues, err := Parse(d.Format, d.Tool, ex.Output, root)
	if err != nil {
		t.Fatalf("parse %s (exit %d, stderr %q): %v", d.Tool, ex.Output.Code, ex.Output.Stderr, err)
	}
	return issues
}

// pythonMaterialize writes the set's tree of the driver's configuration
// into a fresh tool directory, as the lint target does before a run;
// Materialize takes the set directory out of the whole tree itself.
func pythonMaterialize(t *testing.T, d Driver, set string) string {
	t.Helper()
	if _, err := fs.Stat(d.Config, set); err != nil {
		t.Fatalf("set %s of the %s config: %v", set, d.Tool, err)
	}
	dir, err := toolchain.Materialize(t.TempDir(), pythonTestKit, set, d.Config)
	if err != nil {
		t.Fatalf("materialize %s: %v", set, err)
	}
	return dir
}

// pythonRoot answers a fresh directory with its symbolic links resolved,
// because the tools print resolved paths and Parse relativizes to root.
func pythonRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve the temporary root: %v", err)
	}
	return root
}

// pythonFixtureTree copies a fixture tree into a fresh root with every
// .esc file decoded and answers the root and its Python files.
func pythonFixtureTree(t *testing.T, src string) (string, []string) {
	t.Helper()
	root := pythonRoot(t)
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		pythonCopy(t, src, path, root)
		return nil
	})
	if err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
	return root, pythonFiles(t, root)
}

func pythonCopy(t *testing.T, src, path, root string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	rel, err := filepath.Rel(src, path)
	if err != nil {
		t.Fatalf("relativize %s: %v", path, err)
	}
	if plain, ok := strings.CutSuffix(rel, proof.EscapedSuffix); ok {
		rel, data = plain, proof.Unescape(data)
	}
	target := filepath.Join(root, rel)
	if err = os.MkdirAll(filepath.Dir(target), pythonDirMode); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(target), err)
	}
	if err = os.WriteFile(target, data, pythonFileMode); err != nil {
		t.Fatalf("write %s: %v", target, err)
	}
}

// pythonFiles answers the slash paths of the .py files under root, sorted.
func pythonFiles(t *testing.T, root string) []string {
	t.Helper()
	prefix := filepath.ToSlash(root) + "/"
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr == nil && !d.IsDir() && filepath.Ext(path) == ".py" {
			files = append(files, strings.TrimPrefix(filepath.ToSlash(path), prefix))
		}
		return walkErr
	})
	if err != nil {
		t.Fatalf("list %s: %v", root, err)
	}
	slices.Sort(files)
	return files
}

func pythonCodes(issues []Issue) map[string]int {
	codes := make(map[string]int, len(issues))
	for _, i := range issues {
		codes[Code(i.Rule)]++
	}
	return codes
}
