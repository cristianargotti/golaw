package toolchain_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hometest"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
	"github.com/cristianargotti/golaw/internal/toolchaintest"
)

// breakVenv leaves the venv's interpreter as a runtime upgrade leaves it:
// a link to a Python that no longer exists, or no file where none links.
func breakVenv(t *testing.T, tools string) string {
	t.Helper()
	python := toolchain.VenvPython(tools, runtime.GOOS)
	if err := os.Remove(python); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		return python
	}
	if err := os.MkdirAll(filepath.Dir(python), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "python3.12"), python); err != nil {
		t.Fatal(err)
	}
	return python
}

func TestFullRefusesAPyPIToolWhoseVenvPythonIsGone(t *testing.T) {
	t.Parallel()
	tools, bin := hometest.New(t).Tools(), t.TempDir()
	runtimeOnPath(t, bin, pythonName(), "Python 3.12.1")
	cfn := mustLookup(t, "cfn-lint")
	toolchaintest.Install(t, tools, cfn, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{})
	if got := resolver(tools, bin).Full(t.Context(), cfn); got.State != toolchain.StateVerified {
		t.Fatalf("near miss: a live venv must verify: %+v", got)
	}
	python := breakVenv(t, tools)
	got := resolver(tools, bin).Full(t.Context(), cfn)
	if got.Usable() || got.State != toolchain.StateMissing || !strings.Contains(got.Detail, python) || !strings.Contains(got.Detail, "golaw setup --surface") {
		t.Errorf("Full = %+v, want missing naming the venv's Python and the fix", got)
	}
}

func TestInstallRebuildsAVenvWhosePythonIsGone(t *testing.T) {
	t.Parallel()
	dir, bin := hometest.New(t).Tools(), t.TempDir()
	runtimeOnPath(t, bin, pythonName(), "Python 3.12.1")
	toolchaintest.Install(t, dir, pypiTool(), runtime.GOOS, runtime.GOARCH, runnertest.Behavior{})
	pip := toolchain.ExecPath(dir, toolchain.Tool{Name: "pip", Distribution: toolchain.DistPyPI, Bin: "pip"}, runtime.GOOS, runtime.GOARCH)
	runnertest.Install(t, filepath.Dir(pip), "pip", runnertest.Behavior{})
	breakVenv(t, dir)
	var out bytes.Buffer
	o := options(dir, serve(t, nil), &out, []surface.Language{surface.YAML}, pypiTool())
	o.Env = []string{"PATH=" + bin}
	if receipts, err := toolchain.Install(t.Context(), o); err != nil || len(receipts) != 1 {
		t.Fatalf("Install = %+v, %v", receipts, err)
	}
	venv := toolchain.PythonDir(dir)
	calls := runnertest.Calls(t, bin, pythonName())
	if !slices.ContainsFunc(calls, func(c []string) bool { return slices.Equal(c, []string{"-m", "venv", "--clear", venv}) }) {
		t.Errorf("python calls = %v, want the venv rebuilt with --clear", calls)
	}
	if strings.Contains(out.String(), "já estava") || len(runnertest.Calls(t, filepath.Dir(pip), "pip")) != 1 {
		t.Errorf("a venv without its Python was kept: %q", out.String())
	}
}
