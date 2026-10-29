package toolchain_test

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hometest"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/toolchain"
	"github.com/cristianargotti/golaw/internal/toolchaintest"
)

func TestARemovedToolIsGoneForBothModes(t *testing.T) {
	t.Parallel()
	tools := hometest.New(t).Tools()
	ruff := mustLookup(t, "ruff")
	path := toolchaintest.Install(t, tools, ruff, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{})
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	r := resolver(tools, t.TempDir())
	for mode, got := range map[string]toolchain.Resolved{"Hook": r.Hook(ruff), "Full": r.Full(t.Context(), ruff)} {
		if got.State != toolchain.StateMissing || !strings.Contains(got.Detail, "is gone") {
			t.Errorf("%s = %+v, want gone", mode, got)
		}
	}
}

func TestAnUnreceiptedCopyIsNoInstall(t *testing.T) {
	t.Parallel()
	tools := hometest.New(t).Tools()
	ruff := mustLookup(t, "ruff")
	toolchaintest.Unreceipted(t, tools, ruff, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{})
	if got := resolver(tools, t.TempDir()).Hook(ruff); got.State != toolchain.StateMissing || !strings.Contains(got.Detail, "is not installed") {
		t.Errorf("Hook = %+v", got)
	}
}

func TestAnUnreadableReceiptIsMissingNamingItsFile(t *testing.T) {
	t.Parallel()
	tools := hometest.New(t).Tools()
	ruff := mustLookup(t, "ruff")
	toolchaintest.Install(t, tools, ruff, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{})
	if err := os.WriteFile(toolchain.ReceiptPath(tools, "ruff"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := resolver(tools, t.TempDir()).Hook(ruff)
	if got.State != toolchain.StateMissing || !strings.Contains(got.Detail, toolchain.ReceiptPath(tools, "ruff")) {
		t.Errorf("Hook = %+v", got)
	}
}

func TestAReceiptAtAnotherPathIsMissingNamingBoth(t *testing.T) {
	t.Parallel()
	tools := hometest.New(t).Tools()
	ruff := mustLookup(t, "ruff")
	path := toolchaintest.Install(t, tools, ruff, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{})
	receipt, _, err := toolchain.ReadReceipt(tools, "ruff")
	if err != nil {
		t.Fatal(err)
	}
	receipt.Path = t.TempDir()
	if err := toolchain.WriteReceipt(tools, receipt); err != nil {
		t.Fatal(err)
	}
	got := resolver(tools, t.TempDir()).Hook(ruff)
	if got.State != toolchain.StateMissing || !strings.Contains(got.Detail, receipt.Path) || !strings.Contains(got.Detail, path) {
		t.Errorf("Hook = %+v", got)
	}
}

func TestANilRunProbesThroughTheRunner(t *testing.T) {
	t.Parallel()
	bin := t.TempDir()
	ruff := mustLookup(t, "ruff")
	toolchaintest.OnPath(t, bin, ruff, "ruff 0.17.1", runnertest.Behavior{})
	r := resolver(hometest.New(t).Tools(), bin)
	r.Run = nil
	if got := r.Full(t.Context(), ruff); got.State != toolchain.StatePath {
		t.Errorf("Full = %+v", got)
	}
}

func TestAPythonToolNamesItsBounds(t *testing.T) {
	t.Parallel()
	got := resolver(hometest.New(t).Tools(), t.TempDir()).Full(t.Context(), mustLookup(t, "cfn-lint"))
	if got.State != toolchain.StateRuntime || got.Detail != "the PyPI tools need Python 3.10 or newer and older than 3.15 on PATH" {
		t.Errorf("Full = %+v", got)
	}
}

func TestAProbeThatPrintsOnlyStderrIsQuoted(t *testing.T) {
	t.Parallel()
	bin := t.TempDir()
	ruff := mustLookup(t, "ruff")
	toolchaintest.OnPath(t, bin, ruff, "", runnertest.Behavior{})
	runnertest.Install(t, bin, "ruff", runnertest.Behavior{Probe: []string{"unused"}, Stderr: "ruff: broken install\n", Code: 2})
	got := resolver(hometest.New(t).Tools(), bin).Full(t.Context(), ruff)
	if got.State != toolchain.StateMissing || !strings.Contains(got.Detail, "ruff: broken install") {
		t.Errorf("Full = %+v", got)
	}
}
