package setup_test

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/setup"
)

const restored = "setup put the git hooks back as they were before lefthook install"

// TestSetupFailureAfterLefthookPutsTheHooksBack fails setup after lefthook
// install ran: a lefthook that creates two of the three hooks, one that
// exits after the first, and an output that fails. Only a setup that
// succeeds arms a rung, so each failure leaves the hooks directory as
// setup found it, a foreign hook back in place and no backup beside it,
// and names the restoration.
//
//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupFailureAfterLefthookPutsTheHooksBack(t *testing.T) {
	for _, step := range []string{"two hooks created", "install exit", "output failure"} {
		for _, foreign := range []bool{false, true} {
			name := step
			if foreign {
				name += " over a foreign hook"
			}
			t.Run(name, func(t *testing.T) { requireRestored(t, step, foreign) })
		}
	}
}

// requireRestored fails setup at a step past lefthook install and requires
// the hooks directory as setup found it, no receipt, the restoration named,
// and a setup that arms once the step no longer fails.
func requireRestored(t *testing.T, step string, foreign bool) {
	t.Helper()
	options, deps, probe := setupFixture(t)
	if foreign {
		setupWrite(t, filepath.Join(probe.hooks, "pre-commit"), foreignHook)
	}
	before := hooksState(t, probe.hooks)
	plan, err := setup.PlanInstall(t.Context(), options, deps)
	if err != nil {
		t.Fatal(err)
	}
	err = setup.Apply(t.Context(), plan, failAfterLefthook(probe, step), deps)
	if err == nil || !strings.Contains(err.Error(), restored) {
		t.Fatalf("failed setup = %v", err)
	}
	if after := hooksState(t, probe.hooks); after != before {
		t.Fatalf("hooks after the failure = %q, before = %q", after, before)
	}
	if _, readErr := setup.ReadReceipt(plan.ReceiptPath); !errors.Is(readErr, fs.ErrNotExist) {
		t.Fatalf("receipt after the failure = %v", readErr)
	}
	probe.partial, probe.failHook = false, false
	setupSafetyArm(t, options, deps)
}

// TestSetupRefusalBeforeLefthookTouchesNoHook changes a hook after the
// plan: setup refuses before lefthook install and leaves the change.
//
//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupRefusalBeforeLefthookTouchesNoHook(t *testing.T) {
	options, deps, probe := setupFixture(t)
	plan, err := setup.PlanInstall(t.Context(), options, deps)
	if err != nil {
		t.Fatal(err)
	}
	setupWrite(t, filepath.Join(probe.hooks, "pre-push"), foreignHook)
	err = setup.Apply(t.Context(), plan, io.Discard, deps)
	if err == nil || strings.Contains(err.Error(), restored) {
		t.Fatalf("changed hook = %v", err)
	}
	if data, readErr := os.ReadFile(filepath.Join(probe.hooks, "pre-push")); readErr != nil || string(data) != foreignHook {
		t.Fatalf("the hook written after the plan = %q, %v", data, readErr)
	}
	setupSafetyNoInstalls(t, probe)
}

// failAfterLefthook makes setup fail at one step past lefthook install and
// answers the output to apply into: a lefthook that creates two of the
// three hooks, one that exits after the first, or an output that fails.
func failAfterLefthook(probe *setupProbe, step string) io.Writer {
	switch step {
	case "two hooks created":
		probe.partial = true
	case "install exit":
		probe.failHook = true
	case "output failure":
		return setupBrokenWriter{}
	}
	return io.Discard
}

// hooksState renders every file of the hooks directory with its bytes.
func hooksState(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	var state strings.Builder
	for _, entry := range entries {
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			t.Fatal(readErr)
		}
		state.WriteString(entry.Name() + "=" + string(data) + ";")
	}
	return state.String()
}
