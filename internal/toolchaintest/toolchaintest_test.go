package toolchaintest_test

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hometest"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/toolchain"
	"github.com/cristianargotti/golaw/internal/toolchaintest"
)

func TestInstallPutsAFakeAtTheLayoutPathWithItsReceipt(t *testing.T) {
	t.Parallel()
	tools := hometest.New(t).Tools()
	hadolint, _ := toolchain.Lookup("hadolint")
	path := toolchaintest.Install(t, tools, hadolint, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{Stdout: "[]\n", Code: 1})
	if path != toolchain.ExecPath(tools, hadolint, runtime.GOOS, runtime.GOARCH) {
		t.Errorf("fake at %s", path)
	}
	receipt, found, err := toolchain.ReadReceipt(tools, "hadolint")
	if err != nil || !found || receipt.Path != path || receipt.Version != "2.15.1" {
		t.Fatalf("receipt = %+v, %t, %v", receipt, found, err)
	}
	probe, err := runner.Run(t.Context(), runner.Command{Name: path, Args: hadolint.Probe})
	if err != nil || string(probe.Stdout) != "hadolint 2.15.1\n" {
		t.Errorf("probe = %q, %v", probe.Stdout, err)
	}
	out, err := runner.Run(t.Context(), runner.Command{Name: path, Args: []string{"--format", "json", "-"}})
	if err != nil || out.Code != 1 || string(out.Stdout) != "[]\n" {
		t.Errorf("answer = %+v, %v", out, err)
	}
}

func TestOnPathNamesAnNPMToolByItsExecutable(t *testing.T) {
	t.Parallel()
	bin := t.TempDir()
	eslint, _ := toolchain.Lookup("eslint")
	path := toolchaintest.OnPath(t, bin, eslint, "v10.13.0", runnertest.Behavior{})
	want := filepath.Join(bin, "eslint")
	if runtime.GOOS == "windows" {
		want += ".exe"
	}
	if path != want {
		t.Errorf("OnPath = %s, want %s", path, want)
	}
	if _, found, err := toolchain.ReadReceipt(hometest.New(t).Tools(), "eslint"); found || err != nil {
		t.Errorf("OnPath wrote a receipt: %t, %v", found, err)
	}
}

func TestUnreceiptedLeavesNoReceipt(t *testing.T) {
	t.Parallel()
	tools := hometest.New(t).Tools()
	ruff, _ := toolchain.Lookup("ruff")
	toolchaintest.Unreceipted(t, tools, ruff, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{})
	if _, found, err := toolchain.ReadReceipt(tools, "ruff"); found || err != nil {
		t.Errorf("receipt found=%t, %v", found, err)
	}
}

// TestInstallPlaysAPlainScriptToolAsPipLeavesIt: checkov ships a plain #!
// script, which pip copies with no .exe on Windows, so its fake is such a
// script at the layout path and answers through toolchain.Exec on every OS.
func TestInstallPlaysAPlainScriptToolAsPipLeavesIt(t *testing.T) {
	t.Parallel()
	tools := hometest.New(t).Tools()
	checkov, _ := toolchain.Lookup("checkov")
	path := toolchaintest.Install(t, tools, checkov, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{Stdout: "{}\n"})
	if path != toolchain.ExecPath(tools, checkov, runtime.GOOS, runtime.GOARCH) {
		t.Errorf("fake at %s", path)
	}
	if body, err := os.ReadFile(path); err != nil || !strings.HasPrefix(string(body), "#!") {
		t.Fatalf("the fake at %s is no plain #! script: %v", path, err)
	}
	probe, err := toolchain.Exec(t.Context(), toolchain.Request{Path: path, Args: checkov.Probe})
	if err != nil || string(probe.Output.Stdout) != "checkov 3.3.30\n" {
		t.Errorf("probe = %+v, %v", probe, err)
	}
	out, err := toolchain.Exec(t.Context(), toolchain.Request{Path: path, Args: []string{"-o", "json"}})
	if err != nil || string(out.Output.Stdout) != "{}\n" {
		t.Errorf("answer = %+v, %v", out, err)
	}
	calls := toolchaintest.Calls(t, path)
	if len(calls) != 2 || !slices.Equal(calls[0], checkov.Probe) || !slices.Equal(calls[1], []string{"-o", "json"}) {
		t.Errorf("calls = %q, want the tool's own arguments", calls)
	}
}
