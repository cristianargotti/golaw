package adapterkit_test

import (
	"bufio"
	"bytes"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/support"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

const sliceModule = "example.com/slice"

// supportFiles renders the support kind over the growth fixture, by slash path.
func supportFiles(t *testing.T) map[string][]byte {
	t.Helper()
	in := genkind.Input{
		Module: sliceModule, Version: "0.3.0", Policy: model.DefaultGenerate(),
		Snapshot:  gentest.Snapshot(t, gentest.Contracts()),
		Manifest:  manifest.Manifest{Schema: manifest.Schema},
		Contracts: gentest.FakeContracts{},
	}
	out, err := support.Kind().New(t.Context(), in, genkind.Request{})
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte, len(out.Regenerable))
	for _, f := range out.Regenerable {
		files[f.Path] = f.Content
	}
	return files
}

// requireLaws fails for each law of M7.10.2 a rendered support file breaks.
func requireLaws(t *testing.T, p string, content []byte) {
	t.Helper()
	header, ok := genheader.Parse(content)
	if !ok || header.Kind != genkind.KindSupport || header.Template != 1 {
		t.Errorf("%s: header %+v, %v", p, header, ok)
	}
	for _, banned := range []string{"http:" + "//", "nolint", "NOSONAR", "0o644", "0o755", "context.Background()", "context.TO" + "DO()", "\r"} {
		if bytes.Contains(content, []byte(banned)) {
			t.Errorf("%s holds %q", p, banned)
		}
	}
	file, err := parser.ParseFile(token.NewFileSet(), p, content, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("%s: %v", p, err)
	}
	for _, spec := range file.Imports {
		imported := strings.Trim(spec.Path.Value, `"`)
		first, _, _ := strings.Cut(imported, "/")
		if imported == "regexp" || strings.Contains(first, ".") {
			t.Errorf("%s imports %s: the support package is standard library only", p, imported)
		}
	}
	if lines := bytes.Count(content, []byte("\n")); lines > 300 {
		t.Errorf("%s has %d lines", p, lines)
	}
}

// requireScanClean runs the goast family over the files and fails on an
// enforced finding in one of paths.
func requireScanClean(t *testing.T, files map[string][]byte, paths ...string) {
	t.Helper()
	tree := proof.NewTree(files)
	for _, c := range goast.Checks() {
		res, err := c.Run(t.Context(), model.Input{
			Mode: model.ModeScan, Policy: proof.Policy(), Snapshot: tree, Targets: tree.Files(),
			Options: c.Spec().Options.Defaults(),
		})
		if err != nil {
			t.Fatalf("%s: %v", c.Spec().ID, err)
		}
		for i := range res.Findings {
			f := &res.Findings[i]
			if f.Enforced() && slices.Contains(paths, f.Path) {
				t.Errorf("%s: %s:%d %s", c.Spec().ID, f.Path, f.Line, f.Message)
			}
		}
	}
}

// requireCovered writes the files into a module, vets it, runs the tests of
// pattern offline with a coverage profile and fails on a block of names
// that no test ran.
func requireCovered(t *testing.T, files map[string][]byte, pattern string, names ...string) {
	t.Helper()
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go not on PATH, the support package cannot be built: %v", err)
	}
	dir := t.TempDir()
	module := maps.Clone(files)
	module["go.mod"] = []byte("module " + sliceModule + "\n\ngo 1.27\n")
	for p, content := range module {
		target := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	profile := filepath.Join(dir, "cover.out")
	goTool(t, gobin, dir, "vet", "./...")
	goTool(t, gobin, dir, "test", "-count=1", "-covermode=atomic", "-coverprofile="+profile, pattern)
	if blocks := zeroBlocks(t, profile, names); len(blocks) > 0 {
		t.Errorf("blocks no test ran: %v", blocks)
	}
}

func goTool(t *testing.T, gobin, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), gobin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local", "GOPROXY=off", "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func zeroBlocks(t *testing.T, profile string, names []string) []string {
	t.Helper()
	data, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 3 || fields[2] != "0" {
			continue
		}
		file, _, _ := strings.Cut(fields[0], ":")
		for _, name := range names {
			if strings.HasSuffix(file, "/"+name) {
				out = append(out, fields[0])
			}
		}
	}
	return out
}
