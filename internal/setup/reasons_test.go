package setup_test

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/setup"
)

// TestSetupReasonsNameTheGitHooks fails lefthook at each step setup hands
// it: the reason names the git hooks setup arms, never the agent plugin's.
//
//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupReasonsNameTheGitHooks(t *testing.T) {
	for _, step := range []string{"install exit", "hook not created", "uninstall exit"} {
		t.Run(step, func(t *testing.T) {
			options, deps, probe := setupFixture(t)
			plan, deps := failGitHooks(t, options, deps, probe, step)
			err := setup.Apply(t.Context(), plan, io.Discard, deps)
			if !errors.Is(err, setup.ErrGitHooks) || strings.Contains(err.Error(), install.ErrHooks.Error()) {
				t.Fatalf("git hooks reason = %v", err)
			}
		})
	}
}

// failGitHooks makes lefthook fail at one step, its install exit, a hook it
// did not create or its uninstall exit, and answers the plan to apply.
func failGitHooks(
	t *testing.T, options setup.Options, deps setup.Dependencies, probe *setupProbe, step string,
) (setup.Plan, setup.Dependencies) {
	t.Helper()
	switch step {
	case "install exit":
		probe.failHook = true
	case "hook not created":
		run := deps.Run
		deps.Run = func(ctx context.Context, command runner.Command) (runner.Output, error) {
			out, err := run(ctx, command)
			if command.Name == "lefthook" && setupSafetyInstall(command) {
				if removeErr := os.Remove(filepath.Join(probe.hooks, "pre-push")); removeErr != nil {
					t.Fatal(removeErr)
				}
			}
			return out, err
		}
	case "uninstall exit":
		armedReceipt(t, options, deps)
		probe.failUndo = true
		plan, err := setup.PlanUninstall(t.Context(), options, deps)
		if err != nil {
			t.Fatal(err)
		}
		return plan, deps
	}
	plan, err := setup.PlanInstall(t.Context(), options, deps)
	if err != nil {
		t.Fatal(err)
	}
	return plan, deps
}

// TestSetupNamesTheFixForAHookThatIsNotARegularFile links a hook to a
// versioned script, or leaves a directory in its place: the refusal names
// moving it aside, and setup arms once it is aside.
//
//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupNamesTheFixForAHookThatIsNotARegularFile(t *testing.T) {
	for _, kind := range []string{"link", "directory"} {
		t.Run(kind, func(t *testing.T) {
			options, deps, probe := setupFixture(t)
			path := filepath.Join(probe.hooks, "pre-commit")
			occupyHook(t, path, kind)
			_, err := setup.PlanInstall(t.Context(), options, deps)
			if !errors.Is(err, install.ErrPath) || !strings.Contains(err.Error(), "move it aside, then run golaw setup again") {
				t.Fatalf("hook that is not a regular file = %v", err)
			}
			if renameErr := os.Rename(path, path+".aside"); renameErr != nil {
				t.Fatal(renameErr)
			}
			setupSafetyArm(t, options, deps)
		})
	}
}

// occupyHook puts a link to a versioned script, or a directory, where
// setup reads a hook.
func occupyHook(t *testing.T, path, kind string) {
	t.Helper()
	if kind == "directory" {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		return
	}
	script := filepath.Join(t.TempDir(), "pre-commit")
	setupWrite(t, script, foreignHook)
	if err := os.Symlink(script, path); err != nil {
		t.Fatal(err)
	}
}

// TestUninstallNamesTheStepsForAnEditedHook edits an owned hook after setup
// armed it: the receipt keeps only the hook's hash, so the reason names
// re-arming before the uninstall, and those steps uninstall.
//
//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestUninstallNamesTheStepsForAnEditedHook(t *testing.T) {
	options, deps, probe := setupFixture(t)
	receiptPath, _ := armedReceipt(t, options, deps)
	setupWrite(t, filepath.Join(probe.hooks, "pre-commit"), "local hook\n")
	const steps = "copy your edits elsewhere, run golaw setup to re-arm the hook, then run golaw setup --uninstall"
	if _, err := setup.PlanUninstall(t.Context(), options, deps); !errors.Is(err, install.ErrChanged) || !strings.Contains(err.Error(), steps) {
		t.Fatalf("edited hook = %v", err)
	}
	setupSafetyArm(t, options, deps)
	plan, err := setup.PlanUninstall(t.Context(), options, deps)
	if err != nil {
		t.Fatal(err)
	}
	if applyErr := setup.Apply(t.Context(), plan, io.Discard, deps); applyErr != nil {
		t.Fatal(applyErr)
	}
	if _, readErr := setup.ReadReceipt(receiptPath); !errors.Is(readErr, fs.ErrNotExist) {
		t.Fatalf("receipt after the steps = %v", readErr)
	}
}
