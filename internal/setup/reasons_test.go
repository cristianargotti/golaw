package setup_test

import (
	"context"
	"errors"
	"io"
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
