package setup_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/setup"
)

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupStopsBetweenProcesses(t *testing.T) {
	for _, stage := range []string{"tool install", "last verification", "hook install"} {
		t.Run(stage, func(t *testing.T) {
			options, deps, _ := setupFixture(t)
			plan, err := setup.PlanInstall(t.Context(), options, deps)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)
			late, canceled := setupSafetyCancel(ctx, &deps, cancel, stage)
			var out bytes.Buffer
			if err := setup.Apply(ctx, plan, &out, deps); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled apply = %v", err)
			}
			if !*canceled || *late != 0 || out.Len() != 0 {
				t.Fatalf("cancellation: triggered=%v, later calls=%d, output=%q", *canceled, *late, out.String())
			}
			if _, err := setup.ReadReceipt(plan.ReceiptPath); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("receipt after cancellation = %v", err)
			}
		})
	}
}

func setupSafetyCancel(ctx context.Context, deps *setup.Dependencies, cancel context.CancelFunc, stage string) (*int, *bool) {
	late, canceled := 0, false
	run, verify := deps.Run, deps.Verify
	deps.Run = func(callCtx context.Context, command runner.Command) (runner.Output, error) {
		if ctx.Err() != nil {
			late++
		}
		out, err := run(callCtx, command)
		name := strings.TrimSuffix(filepath.Base(command.Name), ".exe")
		if setupSafetyInstall(command) && (stage == "tool install" && name == "go" || stage == "hook install" && name == "lefthook") {
			canceled = true
			cancel()
		}
		return out, err
	}
	deps.Verify = func(callCtx context.Context, tool runner.Tool, env []string) (string, error) {
		if ctx.Err() != nil {
			late++
		}
		path, err := verify(callCtx, tool, env)
		if stage == "last verification" && tool.Name == "lefthook" && err == nil {
			canceled = true
			cancel()
		}
		return path, err
	}
	return &late, &canceled
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupSeparatesGitAndUserEnvironments(t *testing.T) {
	options, deps, probe := setupFixture(t)
	gitDir := probe.repo.GitDir()
	hookEnv := []string{
		"GIT_DIR=" + gitDir, "GIT_WORK_TREE=" + options.Root,
		"GIT_INDEX_FILE=" + filepath.Join(gitDir, "index"), "GIT_COMMON_DIR=" + gitDir, "GIT_PREFIX=kept/",
	}
	options.Env = append(options.Env, hookEnv...)
	options.Env = append(options.Env, "GOBIN="+filepath.Join(t.TempDir(), "old-bin"), "SETUP_EDGE_ENV=kept")
	before := slices.Clone(options.Env)
	run, verify := deps.Run, deps.Verify
	deps.Run = func(ctx context.Context, command runner.Command) (runner.Output, error) {
		if strings.TrimSuffix(filepath.Base(command.Name), ".exe") == "git" {
			for _, entry := range hookEnv {
				if !slices.Contains(command.Env, entry) {
					t.Errorf("git lost %s", entry)
				}
			}
		} else {
			setupSafetyUserEnv(t, command.Env)
		}
		if filepath.Base(command.Name) == "go" || filepath.Base(command.Name) == "go.exe" {
			setupSafetyGOBIN(t, command.Env, options.BinDir)
		}
		return run(ctx, command)
	}
	deps.Verify = func(ctx context.Context, tool runner.Tool, env []string) (string, error) {
		setupSafetyUserEnv(t, env)
		return verify(ctx, tool, env)
	}
	setupSafetyArm(t, options, deps)
	if !slices.Equal(options.Env, before) {
		t.Fatal("setup changed the supplied environment")
	}
}

func setupSafetyUserEnv(t *testing.T, env []string) {
	t.Helper()
	if !slices.Contains(env, "SETUP_EDGE_ENV=kept") {
		t.Fatal("user command lost the supplied environment")
	}
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if slices.ContainsFunc(runner.HookVariables(), func(variable string) bool { return strings.EqualFold(name, variable) }) {
			t.Errorf("user command retained hook variable %s", name)
		}
	}
}

func setupSafetyGOBIN(t *testing.T, env []string, bin string) {
	t.Helper()
	count := 0
	for _, entry := range env {
		name, value, _ := strings.Cut(entry, "=")
		if strings.EqualFold(name, "GOBIN") {
			count++
			if value != bin {
				t.Errorf("installation GOBIN = %q, want %q", value, bin)
			}
		}
	}
	if count != 1 {
		t.Fatalf("installation has %d GOBIN entries", count)
	}
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupFailedRearmRemovesStaleReceiptFirst(t *testing.T) {
	options, deps, probe := setupFixture(t)
	installed := setupSafetyArm(t, options, deps)
	probe.ready["gitleaks"], probe.failTool = false, true
	run, observed := deps.Run, false
	deps.Run = func(ctx context.Context, command runner.Command) (runner.Output, error) {
		if setupSafetyInstall(command) && strings.TrimSuffix(filepath.Base(command.Name), ".exe") == "go" {
			observed = true
			if _, err := setup.ReadReceipt(installed.ReceiptPath); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("stale receipt before failed installation = %v", err)
			}
		}
		return run(ctx, command)
	}
	plan, err := setup.PlanInstall(t.Context(), options, deps)
	if err != nil {
		t.Fatal(err)
	}
	if err := setup.Apply(t.Context(), plan, io.Discard, deps); err == nil {
		t.Fatal("failed tool installation succeeded")
	}
	if !observed {
		t.Fatal("failed rearm did not reach tool installation")
	}
	if _, err := setup.ReadReceipt(plan.ReceiptPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("stale receipt after failure = %v", err)
	}
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupNoopKeepsReceiptBytesAndArmedAt(t *testing.T) {
	options, deps, probe := setupFixture(t)
	installed := setupSafetyArm(t, options, deps)
	before, err := os.ReadFile(installed.ReceiptPath)
	if err != nil {
		t.Fatal(err)
	}
	before = append(append([]byte(" \n"), before...), '\n', '\t')
	if writeErr := os.WriteFile(installed.ReceiptPath, before, 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	options.Now = options.Now.Add(24 * time.Hour)
	verify, seen := deps.Verify, make(map[string]bool)
	deps.Verify = func(ctx context.Context, tool runner.Tool, env []string) (string, error) {
		seen[tool.Name] = true
		return verify(ctx, tool, env)
	}
	probe.calls = nil
	plan, err := setup.PlanInstall(t.Context(), options, deps)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if applyErr := setup.Apply(t.Context(), plan, &out, deps); applyErr != nil {
		t.Fatal(applyErr)
	}
	after, err := os.ReadFile(plan.ReceiptPath)
	if err != nil || !bytes.Equal(before, after) || out.Len() != 0 {
		t.Fatalf("no-op changed receipt or output: %v, %q", err, out.String())
	}
	receipt, err := setup.ReadReceipt(plan.ReceiptPath)
	if err != nil || !receipt.ArmedAt.Equal(options.Now.Add(-24*time.Hour)) {
		t.Fatalf("no-op changed ArmedAt: %v, %v", receipt.ArmedAt, err)
	}
	setupSafetyNoInstalls(t, probe)
	for _, tool := range runner.Toolchain() {
		if !seen[tool.Name] {
			t.Errorf("no-op did not verify %s", tool.Name)
		}
	}
}

func setupSafetyArm(t *testing.T, options setup.Options, deps setup.Dependencies) setup.Plan {
	t.Helper()
	plan, err := setup.PlanInstall(t.Context(), options, deps)
	if err != nil {
		t.Fatal(err)
	}
	if err := setup.Apply(t.Context(), plan, io.Discard, deps); err != nil {
		t.Fatal(err)
	}
	return plan
}

func setupSafetyNoInstalls(t *testing.T, probe *setupProbe) {
	t.Helper()
	for _, command := range probe.calls {
		if setupSafetyInstall(command) {
			t.Fatalf("unexpected installation: %s %v", command.Name, command.Args)
		}
	}
}

func setupSafetyInstall(command runner.Command) bool {
	return len(command.Args) != 0 && command.Args[0] == "install"
}
