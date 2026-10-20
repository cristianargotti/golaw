package setup_test

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/setup"
)

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupDefaultRunnerResolvesLinkedGlobalRouting(t *testing.T) {
	options, _, probe := setupFixture(t)
	probe.repo.Commit("chore: prepare linked repository", nil)
	linked := probe.repo.Worktree("linked")
	linked.Write("lefthook.yml", routedLefthook)
	options.Root = linked.Root
	hooks := filepath.Join(linked.Root, "global hooks")
	global := filepath.Join(t.TempDir(), "gitconfig")
	setupWrite(t, global, "[core]\n\thooksPath = "+strconv.Quote(hooks)+"\n")
	options.Env = append(options.Env, "GIT_CONFIG_GLOBAL="+global)
	plan, err := setup.PlanInstall(t.Context(), options, setup.Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Receipt.Root != linked.Root || plan.Receipt.HooksPath != hooks || plan.Receipt.Golaw != probe.golaw {
		t.Fatalf("resolved routing = %+v", plan.Receipt)
	}
	if len(plan.Changes) != 3 || plan.Changes[0].Path != filepath.Join(hooks, "pre-commit") {
		t.Fatalf("planned hooks = %+v", plan.Changes)
	}
	if _, err := setup.ReadReceipt(plan.ReceiptPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("planning wrote a receipt: %v", err)
	}
	if _, err := os.Stat(hooks); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("planning created hooks: %v", err)
	}
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupGitRoutingFailuresPreserveCauses(t *testing.T) {
	for _, tc := range []struct {
		name, argument string
		output         runner.Output
		cause, want    error
	}{
		{"git start", "--absolute-git-dir", runner.Output{}, io.ErrClosedPipe, io.ErrClosedPipe},
		{"git exit", "--absolute-git-dir", runner.Output{Code: 128, Stderr: []byte("git failed\n")}, nil, install.ErrPath},
		{"relative directory", "--absolute-git-dir", runner.Output{Stdout: []byte("relative\n")}, nil, install.ErrPath},
		{"hooks start", "--git-path", runner.Output{}, io.ErrClosedPipe, io.ErrClosedPipe},
		{"config start", "core.hooksPath", runner.Output{}, io.ErrClosedPipe, io.ErrClosedPipe},
		{"config exit", "core.hooksPath", runner.Output{Code: 128, Stderr: []byte("configuration failed\n")}, nil, install.ErrPath},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options, deps, probe := setupFixture(t)
			run, observed := deps.Run, false
			deps.Run = func(ctx context.Context, command runner.Command) (runner.Output, error) {
				if slices.Contains(command.Args, tc.argument) {
					observed = true
					return tc.output, tc.cause
				}
				return run(ctx, command)
			}
			_, err := setup.PlanInstall(t.Context(), options, deps)
			if !observed || !errors.Is(err, tc.want) {
				t.Fatalf("routing failure = %v, observed=%v, want %v", err, observed, tc.want)
			}
			setupSafetyNoInstalls(t, probe)
		})
	}
}

func TestSetupDefaultRunnerPreservesGitStartFailure(t *testing.T) {
	options, _, probe := setupFixture(t)
	t.Setenv("PATH", options.BinDir)
	_, err := setup.PlanInstall(t.Context(), options, setup.Dependencies{})
	if !errors.Is(err, runner.ErrStart) {
		t.Fatalf("missing git = %v", err)
	}
	setupSafetyNoInstalls(t, probe)
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupRefusesNonregularLefthookAssets(t *testing.T) {
	for _, kind := range []string{"directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			options, deps, probe := setupFixture(t)
			path := filepath.Join(options.Root, "lefthook.yml")
			setupNonregularAsset(t, path, kind)
			_, err := setup.PlanInstall(t.Context(), options, deps)
			if !errors.Is(err, install.ErrPath) || !strings.Contains(err.Error(), "golaw init --apply") {
				t.Fatalf("nonregular asset = %v", err)
			}
			setupSafetyNoInstalls(t, probe)
		})
	}
}

func setupNonregularAsset(t *testing.T, path, kind string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if kind == "directory" {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		return
	}
	target := filepath.Join(t.TempDir(), "lefthook.yml")
	setupWrite(t, target, "min_version: 2.1.0\n")
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}
