package setup_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/setup"
)

// installCase is one row of an installation: what the fixture holds
// before setup and which step of the first apply fails.
type installCase struct {
	name                      string
	prepared, old, tool, hook bool
	foreign                   bool
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupInstallsAndIsIdempotent(t *testing.T) {
	for _, tc := range []installCase{
		{name: "clean"},
		{name: "already pinned", prepared: true},
		{name: "replace another version", old: true},
		{name: "interrupted tool installation", tool: true},
		{name: "interrupted hook installation", hook: true},
		{name: "foreign hook renamed", foreign: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options, deps, probe := setupFixture(t)
			prepareInstall(t, probe, tc)
			var out bytes.Buffer
			plan := applyInstall(t, options, deps, probe, tc, &out)
			requireInstalled(t, plan, probe, tc, out.String())
			requireIdempotent(t, options, deps, probe, plan.ReceiptPath)
		})
	}
}

// prepareInstall sets the fixture of a row: an old toolchain, every tool
// pinned already, a foreign pre-commit hook, and the step that fails.
func prepareInstall(t *testing.T, probe *setupProbe, tc installCase) {
	t.Helper()
	probe.old = tc.old
	if tc.prepared {
		for _, tool := range runner.Toolchain() {
			probe.ready[tool.Name] = true
		}
	}
	if tc.foreign {
		setupWrite(t, filepath.Join(probe.hooks, "pre-commit"), foreignHook)
	}
	probe.failTool, probe.failHook = tc.tool, tc.hook
}

// applyInstall plans and applies setup; when the first apply of a row
// fails, it requires no receipt, then plans and applies again.
func applyInstall(
	t *testing.T, options setup.Options, deps setup.Dependencies,
	probe *setupProbe, tc installCase, out *bytes.Buffer,
) setup.Plan {
	t.Helper()
	plan, err := setup.PlanInstall(t.Context(), options, deps)
	if err != nil {
		t.Fatal(err)
	}
	err = setup.Apply(t.Context(), plan, out, deps)
	if tc.tool || tc.hook {
		if err == nil {
			t.Fatal("failed installation reported success")
		}
		if _, readErr := setup.ReadReceipt(plan.ReceiptPath); !errors.Is(readErr, fs.ErrNotExist) {
			t.Fatalf("completed receipt after failure: %v", readErr)
		}
		probe.failTool, probe.failHook = false, false
		plan, err = setup.PlanInstall(t.Context(), options, deps)
		if err != nil {
			t.Fatal(err)
		}
		err = setup.Apply(t.Context(), plan, out, deps)
	}
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

// requireInstalled requires the receipt to record four tools, three hooks
// at their hashes and the golaw path, a foreign hook kept aside and named,
// and the armed diff in the output.
func requireInstalled(t *testing.T, plan setup.Plan, probe *setupProbe, tc installCase, out string) {
	t.Helper()
	receipt, err := setup.ReadReceipt(plan.ReceiptPath)
	if err != nil || len(receipt.Tools) != 4 || len(receipt.Hooks) != 3 || receipt.Golaw != probe.golaw {
		t.Fatalf("receipt = %+v, %v", receipt, err)
	}
	for _, artifact := range receipt.Hooks {
		data, readErr := os.ReadFile(artifact.Path)
		if readErr != nil || install.Digest(data) != artifact.SHA256 {
			t.Fatalf("hook hash: %s: %v", artifact.Path, readErr)
		}
		if strings.HasSuffix(artifact.Path, ".old") {
			t.Fatalf("foreign backup recorded as owned: %s", artifact.Path)
		}
	}
	if tc.foreign {
		backup := filepath.Join(probe.hooks, "pre-commit.old")
		data, readErr := os.ReadFile(backup)
		if readErr != nil || string(data) != foreignHook || !strings.Contains(out, backup) {
			t.Fatalf("foreign backup = %q, %v; output = %q", data, readErr, out)
		}
	}
	if !strings.Contains(out, "+++ ") {
		t.Fatalf("missing armed diff: %q", out)
	}
}

// requireIdempotent requires a second setup to install nothing, keep the
// receipt bytes and print nothing.
func requireIdempotent(
	t *testing.T, options setup.Options, deps setup.Dependencies,
	probe *setupProbe, receiptPath string,
) {
	t.Helper()
	before, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	probe.calls = nil
	var out bytes.Buffer
	again, err := setup.PlanInstall(t.Context(), options, deps)
	if err != nil {
		t.Fatal(err)
	}
	if applyErr := setup.Apply(t.Context(), again, &out, deps); applyErr != nil {
		t.Fatal(applyErr)
	}
	after, err := os.ReadFile(receiptPath)
	if err != nil || !bytes.Equal(before, after) || out.Len() != 0 {
		t.Fatalf("second setup changed receipt or output: %v, %q", err, out.String())
	}
	for _, call := range probe.calls {
		if len(call.Args) != 0 && call.Args[0] == "install" {
			t.Fatalf("idempotent setup installed again: %+v", call)
		}
	}
}
