package port_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/goldentest"
	"github.com/cristianargotti/golaw/internal/gen/kinds/support"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
)

const porttestDir = "internal/gensupport/porttest/"

func TestThePorttestPairIsPartOfTheSupportPackage(t *testing.T) {
	t.Parallel()
	files := supportFiles(t)
	var names []string
	for path := range files {
		if strings.HasPrefix(path, porttestDir) {
			names = append(names, path)
		}
	}
	slices.Sort(names)
	want := []string{porttestDir + "zz_generated_suite.go", porttestDir + "zz_generated_suite_internal_test.go"}
	if !slices.Equal(names, want) {
		t.Fatalf("porttest files = %v, want %v", names, want)
	}
	pair := make(map[string][]byte, len(names))
	for _, name := range names {
		pair[name] = files[name]
		if h, ok := genheader.Parse(files[name]); !ok || h.Kind != genkind.KindSupport || h.Template != 1 {
			t.Errorf("%s header = %+v, %v", name, h, ok)
		}
	}
	for _, law := range goldentest.Laws() {
		for _, violation := range law.Assert(pair) {
			t.Errorf("%s: %s", law.Name, violation)
		}
	}
}

func TestThePorttestPairPassesAndCoversItself(t *testing.T) {
	t.Parallel()
	// The go command puts its own bin directory first on the PATH of a test,
	// so the lookup finds the toolchain running this test.
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go toolchain not found at %s: the golden slices prove the pair instead", os.Getenv("PATH"))
	}
	dir := t.TempDir()
	files := supportFiles(t)
	files["go.mod"] = []byte("module example.com/slice\n\ngo 1.27\n")
	for path, content := range files {
		target := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	out := goRun(t, dir, goBin, "test", "-count=1", "-cover", "./internal/gensupport/porttest/")
	if !strings.Contains(out, "coverage: 100.0% of statements") {
		t.Errorf("go test = %s, want every statement of porttest covered by its own test", out)
	}
	goRun(t, dir, goBin, "vet", "./internal/gensupport/...")
}

func supportFiles(t *testing.T) map[string][]byte {
	t.Helper()
	in := genkind.Input{
		Module: "example.com/slice", Version: "0.3.0", Policy: model.DefaultGenerate(),
		Snapshot: gentest.Snapshot(t, gentest.Contracts()), Manifest: manifest.Manifest{}, Contracts: gentest.FakeContracts{},
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

func goRun(t *testing.T, dir, goBin string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), goBin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local", "GOPROXY=off", "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
