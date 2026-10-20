package setup_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/setup"
)

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupIgnoresUnrelatedBackupPaths(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		owned      bool
	}{
		{"fresh with directory", "directory", false},
		{"fresh with symlink", "symlink", false},
		{"owned bytes with directory", "directory", true},
		{"owned bytes with symlink", "symlink", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupHookStateInstall(t, tc.kind, tc.owned)
		})
	}
}

func setupHookStateInstall(t *testing.T, kind string, owned bool) {
	t.Helper()
	options, deps, probe := setupFixture(t)
	if owned {
		for _, name := range []string{"pre-commit", "commit-msg", "pre-push"} {
			setupWrite(t, filepath.Join(probe.hooks, name), fakeHook)
		}
	}
	backup := filepath.Join(probe.hooks, "pre-commit.old")
	target := setupHookStateBackup(t, backup, kind)
	plan, err := setup.PlanInstall(t.Context(), options, deps)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if applyErr := setup.Apply(t.Context(), plan, &out, deps); applyErr != nil {
		t.Fatalf("unrelated backup prevented setup: %v", applyErr)
	}
	if strings.Contains(out.String(), backup) {
		t.Fatalf("setup claimed unrelated backup: %q", out.String())
	}
	receipt, err := setup.ReadReceipt(plan.ReceiptPath)
	if err != nil || len(receipt.Hooks) != 3 {
		t.Fatalf("completed receipt = %+v, %v", receipt, err)
	}
	setupHookStateCheckBackup(t, backup, target, kind)
}

func setupHookStateBackup(t *testing.T, backup, kind string) string {
	t.Helper()
	if kind == "directory" {
		target := filepath.Join(backup, "unrelated.txt")
		setupWrite(t, target, foreignHook)
		return target
	}
	target := filepath.Join(t.TempDir(), "unrelated.txt")
	setupWrite(t, target, foreignHook)
	if err := os.Symlink(target, backup); err != nil {
		t.Fatal(err)
	}
	return target
}

func setupHookStateCheckBackup(t *testing.T, backup, target, kind string) {
	t.Helper()
	info, err := os.Lstat(backup)
	if err != nil {
		t.Fatal(err)
	}
	if kind == "directory" && !info.IsDir() || kind == "symlink" && info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("unrelated backup changed type: %v", info.Mode())
	}
	if kind == "symlink" {
		if link, linkErr := os.Readlink(backup); linkErr != nil || link != target {
			t.Fatalf("unrelated backup link changed: %q, %v", link, linkErr)
		}
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != foreignHook {
		t.Fatalf("unrelated backup content changed: %q, %v", data, err)
	}
}

//nolint:paralleltest // The fixture changes PATH for executable resolution.
func TestSetupRearmsHookThatLostExecutePermission(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not use Unix execute permission bits")
	}
	options, deps, probe := setupFixture(t)
	setupSafetyArm(t, options, deps)
	path := filepath.Join(probe.hooks, "pre-commit")
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	run, rearmed := deps.Run, false
	deps.Run = func(ctx context.Context, command runner.Command) (runner.Output, error) {
		out, err := run(ctx, command)
		if setupSafetyInstall(command) && strings.TrimSuffix(filepath.Base(command.Name), ".exe") == "lefthook" {
			rearmed = true
			// The fake models lefthook restoring executable permissions on installation.
			if chmodErr := os.Chmod(path, 0o700); chmodErr != nil {
				t.Fatal(chmodErr)
			}
		}
		return out, err
	}
	setupSafetyArm(t, options, deps)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !rearmed || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("hook remained unarmed: lefthook install=%v, mode=%v", rearmed, info.Mode())
	}
}
