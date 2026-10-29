package toolchain_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/hometest"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// placed writes the file a package manager would have written at the
// layout path of t, because the fake npm and pip record their call and
// install nothing.
func placed(t *testing.T, dir string, tool toolchain.Tool) {
	t.Helper()
	path := toolchain.ExecPath(dir, tool, runtime.GOOS, runtime.GOARCH)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("shim of "+tool.Name+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
}

func npmTool() toolchain.Tool {
	return toolchain.Tool{Name: "eslint", Version: "10.13.0", Distribution: toolchain.DistNPM, Languages: []surface.Language{surface.JavaScript}, Probe: []string{"--version"}, Bin: "eslint"}
}

func pypiTool() toolchain.Tool {
	return toolchain.Tool{Name: "yamllint", Version: "1.38.0", Distribution: toolchain.DistPyPI, Languages: []surface.Language{surface.YAML}, Probe: []string{"--version"}, Bin: "yamllint"}
}

func TestInstallRunsNpmCiOverTheEmbeddedLockWithScriptsOff(t *testing.T) {
	t.Parallel()
	dir, bin := hometest.New(t).Tools(), t.TempDir()
	runtimeOnPath(t, bin, "node", "v24.21.0")
	runnertest.Install(t, bin, "npm", runnertest.Behavior{})
	placed(t, dir, npmTool())
	o := options(dir, serve(t, nil), io.Discard, []surface.Language{surface.JavaScript}, npmTool())
	o.Env = []string{"PATH=" + bin}
	receipts, err := toolchain.Install(t.Context(), o)
	if err != nil || len(receipts) != 1 || receipts[0].Name != "eslint" {
		t.Fatalf("Install = %+v, %v", receipts, err)
	}
	calls := runnertest.Calls(t, bin, "npm")
	if len(calls) != 1 || !slices.Equal(calls[0], []string{"ci", "--ignore-scripts", "--no-audit", "--no-fund", "--legacy-peer-deps"}) {
		t.Errorf("npm calls = %v, want npm ci with scripts off and the peer range the lock was written with", calls)
	}
	for _, name := range []string{"package.json", "package-lock.json"} {
		if _, statErr := os.Stat(filepath.Join(toolchain.NodeDir(dir), name)); statErr != nil {
			t.Errorf("%s not written beside node_modules: %v", name, statErr)
		}
	}
}

func TestInstallRefusesTheNpmToolsWithoutNodeAndKeepsTheOthers(t *testing.T) {
	t.Parallel()
	dir, bin := hometest.New(t).Tools(), t.TempDir()
	runnertest.Install(t, bin, "npm", runnertest.Behavior{})
	s := serve(t, map[string][]byte{"/fake": payload()})
	fake := binaryTool("fake", s.URL+"/fake", payload(), "", "", surface.JavaScript)
	o := options(dir, s, io.Discard, []surface.Language{surface.JavaScript}, npmTool(), fake)
	o.Env = []string{"PATH=" + bin}
	receipts, err := toolchain.Install(t.Context(), o)
	if !errors.Is(err, toolchain.ErrRuntime) || len(receipts) != 1 || receipts[0].Name != "fake" {
		t.Fatalf("Install = %+v, %v, want the binary tool and ErrRuntime", receipts, err)
	}
	if calls := runnertest.Calls(t, bin, "npm"); len(calls) != 0 {
		t.Errorf("npm ran without node: %v", calls)
	}
}

func TestInstallBuildsTheVenvAndInstallsTheHashedLock(t *testing.T) {
	t.Parallel()
	dir, bin := hometest.New(t).Tools(), t.TempDir()
	runtimeOnPath(t, bin, pythonName(), "Python 3.12.1")
	venv := toolchain.PythonDir(dir)
	pip := toolchain.ExecPath(dir, toolchain.Tool{Name: "pip", Distribution: toolchain.DistPyPI, Bin: "pip"}, runtime.GOOS, runtime.GOARCH)
	if err := os.MkdirAll(filepath.Dir(pip), 0o700); err != nil {
		t.Fatal(err)
	}
	runnertest.Install(t, filepath.Dir(pip), "pip", runnertest.Behavior{})
	placed(t, dir, pypiTool())
	var out bytes.Buffer
	o := options(dir, serve(t, nil), &out, []surface.Language{surface.YAML}, pypiTool())
	o.Env = []string{"PATH=" + bin}
	receipts, err := toolchain.Install(t.Context(), o)
	if err != nil || len(receipts) != 1 || receipts[0].Name != "yamllint" {
		t.Fatalf("Install = %+v, %v", receipts, err)
	}
	// The runtime probe is a call of its own, before the one venv build.
	if calls := runnertest.Calls(t, bin, pythonName()); len(calls) != 2 || !slices.Equal(calls[0], []string{"--version"}) || !slices.Equal(calls[1], []string{"-m", "venv", venv}) {
		t.Errorf("python calls = %v, want the runtime probe then the venv", calls)
	}
	lock := filepath.Join(filepath.Dir(venv), "requirements.lock")
	want := []string{"install", "--require-hashes", "--no-deps", "--only-binary", ":all:", "--disable-pip-version-check", "-r", lock}
	if calls := runnertest.Calls(t, filepath.Dir(pip), "pip"); len(calls) != 1 || !slices.Equal(calls[0], want) {
		t.Errorf("pip calls = %v, want %v", calls, want)
	}
	if _, statErr := os.Stat(lock); statErr != nil {
		t.Errorf("the lock was not written beside the venv: %v", statErr)
	}
}
