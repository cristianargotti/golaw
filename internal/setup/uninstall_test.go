package setup_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/setup"
)

// uninstallCase is one row of an uninstall: what the fixture holds or
// changes once setup armed it, and whether lefthook uninstall fails.
type uninstallCase struct {
	name                      string
	foreign, changed, missing bool
	routing, fail             bool
}

// uninstallPaths are the files an uninstall row reads: the receipt setup
// wrote, the owned pre-commit hook and the unrelated post-checkout hook.
type uninstallPaths struct {
	receipt, owned, foreign string
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestUninstallPreservesForeignAndChangedFiles(t *testing.T) {
	for _, tc := range []uninstallCase{
		{name: "intact"},
		{name: "foreign hook restored", foreign: true},
		{name: "local edit", changed: true},
		{name: "owned hook already missing", missing: true},
		{name: "routing changed", routing: true},
		{name: "tool failure", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options, deps, probe := setupFixture(t)
			paths := uninstallPaths{
				owned:   filepath.Join(probe.hooks, "pre-commit"),
				foreign: filepath.Join(probe.hooks, "post-checkout"),
			}
			paths.receipt = armForUninstall(t, options, deps, probe, tc, paths)
			uninstallRow(t, options, deps, probe, paths, tc)
		})
	}
}

// armForUninstall arms the fixture with setup and then makes the change of
// the row: a foreign hook before setup, an unrelated hook beside the owned
// ones, a local edit, a missing owned hook, another routing or a failing
// lefthook uninstall. It answers the receipt path setup wrote.
func armForUninstall(
	t *testing.T, options setup.Options, deps setup.Dependencies,
	probe *setupProbe, tc uninstallCase, paths uninstallPaths,
) string {
	t.Helper()
	if tc.foreign {
		setupWrite(t, paths.owned, foreignHook)
	}
	installed, err := setup.PlanInstall(t.Context(), options, deps)
	if err != nil {
		t.Fatal(err)
	}
	if applyErr := setup.Apply(t.Context(), installed, io.Discard, deps); applyErr != nil {
		t.Fatal(applyErr)
	}
	setupWrite(t, paths.foreign, foreignHook)
	if tc.changed {
		setupWrite(t, paths.owned, "local hook\n")
	}
	if tc.missing {
		if removeErr := os.Remove(paths.owned); removeErr != nil {
			t.Fatal(removeErr)
		}
	}
	if tc.routing {
		probe.repo.Git("config", "--local", "core.hooksPath", "custom-hooks")
	}
	probe.failUndo = tc.fail
	return installed.ReceiptPath
}

// uninstallRow uninstalls once and requires the answer of the row: a
// refusal that keeps the receipt for a changed installation, a failure
// that keeps it for a failing tool, and otherwise a clean removal a second
// uninstall leaves alone.
func uninstallRow(
	t *testing.T, options setup.Options, deps setup.Dependencies,
	probe *setupProbe, paths uninstallPaths, tc uninstallCase,
) {
	t.Helper()
	var out bytes.Buffer
	plan, err := setup.PlanUninstall(t.Context(), options, deps)
	if tc.changed || tc.routing {
		if !errors.Is(err, install.ErrChanged) {
			t.Fatalf("changed installation = %v", err)
		}
		requireReceiptKept(t, paths.receipt, "refusal")
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	err = setup.Apply(t.Context(), plan, &out, deps)
	if tc.fail {
		if err == nil {
			t.Fatal("failed uninstall reported success")
		}
		requireReceiptKept(t, paths.receipt, "failure")
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	requireUninstalled(t, probe, paths, tc.foreign)
	if !strings.Contains(out.String(), "--- ") {
		t.Fatalf("missing removal diff: %q", out.String())
	}
	requireEmptyUninstall(t, options, deps, probe)
}

// requireReceiptKept requires the receipt setup wrote to stay readable.
func requireReceiptKept(t *testing.T, path, after string) {
	t.Helper()
	if _, readErr := setup.ReadReceipt(path); readErr != nil {
		t.Fatalf("receipt lost after %s: %v", after, readErr)
	}
}

// requireUninstalled requires the receipt, the owned hooks and the
// checksum gone, a foreign hook restored, and the unrelated hook and every
// tool kept.
func requireUninstalled(t *testing.T, probe *setupProbe, paths uninstallPaths, foreign bool) {
	t.Helper()
	if _, readErr := setup.ReadReceipt(paths.receipt); !errors.Is(readErr, fs.ErrNotExist) {
		t.Fatalf("receipt remains: %v", readErr)
	}
	data, readErr := os.ReadFile(paths.owned)
	if foreign {
		if readErr != nil || string(data) != foreignHook {
			t.Fatalf("restored foreign hook = %q, %v", data, readErr)
		}
	} else if !errors.Is(readErr, fs.ErrNotExist) {
		t.Fatalf("owned hook remains: %v", readErr)
	}
	data, readErr = os.ReadFile(paths.foreign)
	if readErr != nil || string(data) != foreignHook {
		t.Fatalf("unrelated hook = %q, %v", data, readErr)
	}
	if _, statErr := os.Stat(probe.checksum); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("checksum remains: %v", statErr)
	}
	for name, ready := range probe.ready {
		if !ready {
			t.Fatalf("tool removed: %s", name)
		}
	}
}

// requireEmptyUninstall requires a second uninstall to print nothing and
// to leave lefthook alone.
func requireEmptyUninstall(t *testing.T, options setup.Options, deps setup.Dependencies, probe *setupProbe) {
	t.Helper()
	var out bytes.Buffer
	probe.calls = nil
	again, err := setup.PlanUninstall(t.Context(), options, deps)
	if err != nil {
		t.Fatal(err)
	}
	if applyErr := setup.Apply(t.Context(), again, &out, deps); applyErr != nil || out.Len() != 0 {
		t.Fatalf("second uninstall = %v, %q", applyErr, out.String())
	}
	for _, call := range probe.calls {
		if len(call.Args) == 1 && call.Args[0] == "uninstall" {
			t.Fatalf("empty uninstall invoked lefthook: %+v", call)
		}
	}
}
