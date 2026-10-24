package runner_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
)

func startRunnerChild(
	t *testing.T, testName string, vars []string,
) (*exec.Cmd, string) {
	t.Helper()
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ready := filepath.Join(t.TempDir(), "ready")
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, program, "-test.run=^"+testName+"$")
	cmd.Env = append(gittest.Isolated(), vars...)
	cmd.Env = append(cmd.Env, "GOLAW_READY="+ready)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopRunnerChild(t, cmd) })
	for {
		data, readErr := os.ReadFile(ready)
		if readErr == nil && len(data) != 0 {
			return cmd, strings.TrimSpace(string(data))
		}
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			t.Fatal(readErr)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Millisecond):
		}
	}
}

func stopRunnerChild(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if cmd.ProcessState != nil {
		return
	}
	// Windows answers "Access is denied" to a kill that races the child's
	// own exit; the wait below still proves the child gone there.
	if err := cmd.Process.Kill(); err != nil &&
		!errors.Is(err, os.ErrProcessDone) && runtime.GOOS != "windows" {
		t.Fatal(err)
	}
	err := cmd.Wait()
	var exited *exec.ExitError
	if err != nil && !errors.As(err, &exited) {
		t.Fatal(err)
	}
}

func TestLockWaitsForRelease(t *testing.T) {
	t.Parallel()
	path := runner.LockPath(t.TempDir(), "handoff")
	cmd, ready := startRunnerChild(t, "TestLockChild", []string{
		"GOLAW_LOCK_CHILD=" + path,
		"GOLAW_LOCK_DELAY=250ms",
	})
	if ready != "ready" {
		t.Fatalf("child readiness = %q", ready)
	}
	lock, err := runner.Acquire(t.Context(), path, runner.RungWait)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestLockRejectsLinksWithoutChangingTargets(t *testing.T) {
	t.Parallel()
	for _, suffix := range []string{"", ".pid"} {
		t.Run("path"+suffix, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path, target := filepath.Join(dir, "lock"), filepath.Join(dir, "target")
			if err := os.WriteFile(target, []byte("preserve\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, path+suffix); err != nil {
				if runtime.GOOS == "windows" {
					t.Skipf("symlink creation unavailable: %v", err)
				}
				t.Fatal(err)
			}
			requireLockLinkResult(t, path, suffix)
			data, err := os.ReadFile(target)
			if err != nil || string(data) != "preserve\n" {
				t.Fatalf("symlink target = %q, %v", data, err)
			}
		})
	}
}

func requireLockLinkResult(t *testing.T, path, suffix string) {
	t.Helper()
	lock, err := runner.Acquire(t.Context(), path, 0)
	if suffix == "" {
		if !errors.Is(err, os.ErrInvalid) {
			t.Fatalf("symlink lock = %v", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
}
