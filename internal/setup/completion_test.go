package setup_test

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/setup"
)

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupPlanningFailuresDoNotInstall(t *testing.T) {
	for _, invalid := range []string{"canceled", "root", "state directory", "binary directory", "hook directory"} {
		t.Run(invalid, func(t *testing.T) {
			options, deps, probe := setupFixture(t)
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)
			switch invalid {
			case "canceled":
				cancel()
			case "root":
				options.Root = filepath.Join(options.Root, "absent")
			case "state directory":
				options.StateDir = "relative"
			case "binary directory":
				options.BinDir = "relative"
			case "hook directory":
				if err := os.Mkdir(filepath.Join(probe.hooks, "pre-commit"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := setup.PlanInstall(ctx, options, deps); err == nil {
				t.Fatal("invalid plan succeeded")
			}
			setupSafetyNoInstalls(t, probe)
		})
	}
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupDefaultVerifierUsesSuppliedEnvironment(t *testing.T) {
	options, deps, probe := setupFixture(t)
	deps.Verify = nil
	options.Env = append(options.Env, "SETUP_DEFAULT_ENV=supplied")
	for _, tool := range runner.Toolchain() {
		runnertest.Install(t, probe.bin, tool.Name, runnertest.Behavior{Stdout: tool.Version + "\n"})
	}
	run := deps.Run
	deps.Run = func(ctx context.Context, command runner.Command) (runner.Output, error) {
		if !slices.Contains(command.Env, "SETUP_DEFAULT_ENV=supplied") {
			t.Error("supplied environment was lost")
		}
		if command.Name == "go" {
			t.Error("pinned process tools were reinstalled")
		}
		return run(ctx, command)
	}
	plan := setupSafetyArm(t, options, deps)
	receipt, err := setup.ReadReceipt(plan.ReceiptPath)
	if err != nil || len(receipt.Tools) != len(runner.Toolchain()) {
		t.Fatalf("default verification receipt = %+v, %v", receipt, err)
	}
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupCompletionRequiresHookFilesAndWritableReceipt(t *testing.T) {
	for _, incomplete := range []string{"missing hook", "nonregular hook", "receipt parent"} {
		t.Run(incomplete, func(t *testing.T) {
			options, deps, probe := setupFixture(t)
			plan, err := setup.PlanInstall(t.Context(), options, deps)
			if err != nil {
				t.Fatal(err)
			}
			run := deps.Run
			deps.Run = func(ctx context.Context, command runner.Command) (runner.Output, error) {
				out, runErr := run(ctx, command)
				if command.Name == "lefthook" && setupSafetyInstall(command) {
					breakSetupCompletion(t, incomplete, plan, probe)
				}
				return out, runErr
			}
			if applyErr := setup.Apply(t.Context(), plan, io.Discard, deps); applyErr == nil {
				t.Fatal("incomplete installation succeeded")
			}
			if _, readErr := setup.ReadReceipt(plan.ReceiptPath); readErr == nil {
				t.Fatal("incomplete installation published receipt")
			}
		})
	}
}

func breakSetupCompletion(t *testing.T, incomplete string, plan setup.Plan, probe *setupProbe) {
	t.Helper()
	if incomplete == "receipt parent" {
		setupWrite(t, filepath.Dir(plan.ReceiptPath), "occupied")
		return
	}
	path := filepath.Join(probe.hooks, "pre-push")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if incomplete == "nonregular hook" {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
}

type cancelSetupWriter struct{ cancel context.CancelFunc }

func (w cancelSetupWriter) Write(data []byte) (int, error) {
	w.cancel()
	return len(data), nil
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupCancellationDuringOutputLeavesNoReceipt(t *testing.T) {
	options, deps, _ := setupFixture(t)
	plan, err := setup.PlanInstall(t.Context(), options, deps)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	if applyErr := setup.Apply(ctx, plan, cancelSetupWriter{cancel: cancel}, deps); !errors.Is(applyErr, context.Canceled) {
		t.Fatalf("canceled output = %v", applyErr)
	}
	if _, readErr := setup.ReadReceipt(plan.ReceiptPath); !errors.Is(readErr, fs.ErrNotExist) {
		t.Fatalf("receipt after canceled output = %v", readErr)
	}
	plan.Uninstall = true
	if applyErr := setup.Apply(t.Context(), plan, io.Discard, deps); !errors.Is(applyErr, install.ErrPath) {
		t.Fatalf("installation accepted an uninstall plan = %v", applyErr)
	}
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupPreservesLefthookFailureReason(t *testing.T) {
	for _, stream := range []string{"stdout", "stderr"} {
		t.Run(stream, func(t *testing.T) {
			options, deps, _ := setupFixture(t)
			const fix = "git config --unset-all --global core.hooksPath"
			setupFailureStream(&deps, stream, fix)
			plan, err := setup.PlanInstall(t.Context(), options, deps)
			if err != nil {
				t.Fatal(err)
			}
			err = setup.Apply(t.Context(), plan, io.Discard, deps)
			if !errors.Is(err, setup.ErrGitHooks) || !strings.Contains(err.Error(), fix) {
				t.Fatalf("lefthook failure lost its remedy: %v", err)
			}
			if _, readErr := setup.ReadReceipt(plan.ReceiptPath); !errors.Is(readErr, fs.ErrNotExist) {
				t.Fatalf("receipt after lefthook refusal = %v", readErr)
			}
		})
	}
}

func setupFailureStream(deps *setup.Dependencies, stream, reason string) {
	run := deps.Run
	deps.Run = func(ctx context.Context, command runner.Command) (runner.Output, error) {
		if command.Name != "lefthook" || !setupSafetyInstall(command) {
			return run(ctx, command)
		}
		out := runner.Output{Code: 1, Stdout: []byte(reason + "\n")}
		if stream == "stderr" {
			out.Stderr, out.Stdout = out.Stdout, nil
		}
		return out, nil
	}
}
