package toolchain_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hometest"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/toolchain"
	"github.com/cristianargotti/golaw/internal/toolchaintest"
)

func resolver(dir, bin string) toolchain.Resolver {
	return toolchain.Resolver{Dir: dir, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Env: []string{"PATH=" + bin}, Run: runner.Run}
}

// pythonName is the executable the python runtime is found as here.
func pythonName() string {
	if runtime.GOOS == "windows" {
		return "python"
	}
	return "python3"
}

func runtimeOnPath(t *testing.T, bin, name, version string) string {
	t.Helper()
	return runnertest.Install(t, bin, name, runnertest.Behavior{Probe: []string{"--version"}, Version: version})
}

func TestHookTrustsAMatchingReceiptAndStartsNothing(t *testing.T) {
	t.Parallel()
	tools, bin := hometest.New(t).Tools(), t.TempDir()
	ruff := mustLookup(t, "ruff")
	path := toolchaintest.Install(t, tools, ruff, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{Stdout: "[]\n"})
	got := resolver(tools, bin).Hook(ruff)
	if got.State != toolchain.StateReceipt || got.Path != path || !got.Usable() || !strings.Contains(got.Detail, "receipt matches") {
		t.Fatalf("Hook = %+v", got)
	}
	if calls := runnertest.Calls(t, filepath.Dir(path), "ruff"); len(calls) != 0 {
		t.Errorf("the hook resolution started the tool: %v", calls)
	}
}

func TestHookNeverLooksAtPath(t *testing.T) {
	t.Parallel()
	tools, bin := hometest.New(t).Tools(), t.TempDir()
	ruff := mustLookup(t, "ruff")
	toolchaintest.OnPath(t, bin, ruff, "ruff 0.17.1", runnertest.Behavior{})
	got := resolver(tools, bin).Hook(ruff)
	if got.State != toolchain.StateMissing || got.Usable() || !strings.Contains(got.Detail, "run golaw setup --surface") {
		t.Fatalf("Hook = %+v, want missing with the fix", got)
	}
	if calls := runnertest.Calls(t, bin, "ruff"); len(calls) != 0 {
		t.Errorf("the hook probed PATH: %v", calls)
	}
}

func TestHookAnswersDriftAndAStaleReceiptAsUnusable(t *testing.T) {
	t.Parallel()
	ruff := mustLookup(t, "ruff")
	t.Run("a changed size is drift", func(t *testing.T) {
		t.Parallel()
		tools := hometest.New(t).Tools()
		path := toolchaintest.Install(t, tools, ruff, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{})
		file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte{0}); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		if got := resolver(tools, t.TempDir()).Hook(ruff); got.State != toolchain.StateDrift || got.Usable() {
			t.Errorf("Hook = %+v, want drift", got)
		}
	})
	t.Run("a receipt of another pin is no install of this one", func(t *testing.T) {
		t.Parallel()
		tools := hometest.New(t).Tools()
		older := ruff
		older.Version = "0.15.6"
		toolchaintest.Install(t, tools, older, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{})
		got := resolver(tools, t.TempDir()).Hook(ruff)
		if got.State != toolchain.StateMissing || !strings.Contains(got.Detail, "0.15.6") {
			t.Errorf("Hook = %+v, want missing naming the recorded version", got)
		}
	})
}

func TestFullRehashesWhatTheHookTrustsBySizeAndTime(t *testing.T) {
	t.Parallel()
	tools, bin := hometest.New(t).Tools(), t.TempDir()
	ruff := mustLookup(t, "ruff")
	path := toolchaintest.Install(t, tools, ruff, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{})
	if got := resolver(tools, bin).Full(t.Context(), ruff); got.State != toolchain.StateVerified || !strings.Contains(got.Detail, "sha256 verified") {
		t.Fatalf("Full = %+v, want verified", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte{'x'}, int(info.Size())), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if got := resolver(tools, bin).Hook(ruff); got.State != toolchain.StateReceipt {
		t.Errorf("Hook = %+v, want the cheap check to pass", got)
	}
	if got := resolver(tools, bin).Full(t.Context(), ruff); got.State != toolchain.StateDrift || got.Usable() {
		t.Errorf("Full = %+v, want drift after the rehash", got)
	}
}

func TestFullFallsBackToAToolOnPathAtThePinOnly(t *testing.T) {
	t.Parallel()
	ruff := mustLookup(t, "ruff")
	tests := map[string]struct {
		version string // empty installs nothing on PATH
		state   toolchain.State
		detail  string
	}{
		"the pin":                      {"ruff 0.17.1", toolchain.StatePath, "on PATH, version matches, checksum not verified"},
		"another version (near miss)":  {"ruff 0.15.6", toolchain.StateMissing, "0.15.6"},
		"a longer version (near miss)": {"ruff 0.17.10", toolchain.StateMissing, "0.17.10"},
		"nothing anywhere":             {"", toolchain.StateMissing, "run golaw setup --surface"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bin := t.TempDir()
			want := ""
			if tc.version != "" {
				want = toolchaintest.OnPath(t, bin, ruff, tc.version, runnertest.Behavior{})
			}
			got := resolver(hometest.New(t).Tools(), bin).Full(t.Context(), ruff)
			if got.State != tc.state || !strings.Contains(got.Detail, tc.detail) || tc.state == toolchain.StatePath && got.Path != want {
				t.Errorf("Full = %+v, want %s with %q", got, tc.state, tc.detail)
			}
		})
	}
}

func TestFullWithoutAToolsDirectoryNamesTheCause(t *testing.T) {
	t.Parallel()
	got := resolver("", t.TempDir()).Full(t.Context(), mustLookup(t, "hadolint"))
	if got.State != toolchain.StateMissing || !strings.Contains(got.Detail, "no tools directory") {
		t.Errorf("Full = %+v", got)
	}
}

func TestAnNPMToolNeedsItsRuntimeFirst(t *testing.T) {
	t.Parallel()
	eslint := mustLookup(t, "eslint")
	t.Run("no node", func(t *testing.T) {
		t.Parallel()
		got := resolver(hometest.New(t).Tools(), t.TempDir()).Full(t.Context(), eslint)
		if got.State != toolchain.StateRuntime || got.Detail != "the npm tools need Node 22.13 or newer on PATH" {
			t.Errorf("Full = %+v", got)
		}
	})
	t.Run("node below the floor", func(t *testing.T) {
		t.Parallel()
		bin := t.TempDir()
		runtimeOnPath(t, bin, "node", "v20.11.0")
		got := resolver(hometest.New(t).Tools(), bin).Full(t.Context(), eslint)
		if got.State != toolchain.StateRuntime || !strings.Contains(got.Detail, "v20.11.0") {
			t.Errorf("Full = %+v", got)
		}
	})
	t.Run("node at the floor and an installed tool", func(t *testing.T) {
		t.Parallel()
		tools, bin := hometest.New(t).Tools(), t.TempDir()
		runtimeOnPath(t, bin, "node", "v24.21.0")
		toolchaintest.Install(t, tools, eslint, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{})
		if got := resolver(tools, bin).Full(t.Context(), eslint); got.State != toolchain.StateVerified {
			t.Errorf("Full = %+v", got)
		}
	})
}

func TestRuntimeReadsTheVersionEachRuntimePrints(t *testing.T) {
	t.Parallel()
	var node, python toolchain.Runtime
	for _, rt := range toolchain.Runtimes() {
		switch rt.Name {
		case "node":
			node = rt
		case "python":
			python = rt
		}
	}
	tests := map[string]struct {
		rt         toolchain.Runtime
		executable string
		prints     string
		ok         bool
		version    string
	}{
		"node 24":                  {node, "node", "v24.21.0", true, "24.21.0"},
		"node 22.13.0":             {node, "node", "v22.13.0", true, "22.13.0"},
		"node 22.12.0 (near miss)": {node, "node", "v22.12.0", false, "22.12.0"}, // eslint 10 needs 22.13
		"python 3.12":              {python, pythonName(), "Python 3.12.1", true, "3.12.1"},
		"python 3.14":              {python, pythonName(), "Python 3.14.0", true, "3.14.0"},
		"python 3.15 (near miss)":  {python, pythonName(), "Python 3.15.0", false, "3.15.0"}, // cfn-lint refuses it
		"python 3.9 (near miss)":   {python, pythonName(), "Python 3.9.1", false, "3.9.1"},
		"garbage (near miss)":      {node, "node", "not a version", false, ""},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bin := t.TempDir()
			path := runtimeOnPath(t, bin, tc.executable, tc.prints)
			got := resolver(hometest.New(t).Tools(), bin).Runtime(t.Context(), tc.rt)
			if got.OK != tc.ok || got.Version != tc.version || got.Path != path {
				t.Errorf("Runtime = %+v, want ok=%t version %q at %s", got, tc.ok, tc.version, path)
			}
		})
	}
}

func TestNoticeOfAResolvedToolIsTheMissingNotice(t *testing.T) {
	t.Parallel()
	ruff := mustLookup(t, "ruff")
	resolved := toolchain.Resolved{Tool: ruff, State: toolchain.StateMissing}
	agent, person := resolved.Notice("scripts/a.py", "pt-BR")
	wantAgent, wantPerson := toolchain.MissingNotice(ruff, "scripts/a.py", "pt-BR")
	if agent != wantAgent || person != wantPerson {
		t.Errorf("Notice = %q, %q", agent, person)
	}
}
