package runner_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/runner"
)

func TestLockChild(t *testing.T) {
	t.Parallel()
	path, enabled := os.LookupEnv("GOLAW_LOCK_CHILD")
	if !enabled {
		return
	}
	delay, err := time.ParseDuration(os.Getenv("GOLAW_LOCK_DELAY"))
	if err != nil {
		t.Fatal(err)
	}
	lock, err := runner.Acquire(t.Context(), path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		os.Getenv("GOLAW_READY"), []byte("ready\n"), 0o600,
	); err != nil {
		t.Fatal(err)
	}
	time.Sleep(delay)
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestLockContentionAndProcessDeath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		kill bool
	}{
		{"red-live-holder", false},
		{"near-ended-holder", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := runner.LockPath(t.TempDir(), "pre-commit")
			cmd, _ := startRunnerChild(t, "TestLockChild", []string{
				"GOLAW_LOCK_CHILD=" + path,
				"GOLAW_LOCK_DELAY=10s",
			})
			if !tc.kill {
				requireHeldBy(t, path, cmd.Process.Pid)
				return
			}
			stopRunnerChild(t, cmd)
			lock, err := runner.Acquire(t.Context(), path, runner.RungWait)
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := lock.Release(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// requireHeldBy requires a short wait on a lock a live process holds to
// fail with the lock and the process named.
func requireHeldBy(t *testing.T, path string, pid int) {
	t.Helper()
	_, err := runner.Acquire(t.Context(), path, 20*time.Millisecond)
	var held *runner.LockedError
	if !errors.Is(err, runner.ErrLocked) ||
		!errors.As(err, &held) {
		t.Fatalf("contention = %v", err)
	}
	if held.Path != path || held.PID != pid {
		t.Fatalf("holder = %+v", held)
	}
	want := fmt.Sprintf(
		"lock %s is held by process %d",
		path, pid,
	)
	if held.Error() != want {
		t.Fatalf("reason = %q", held.Error())
	}
}

func TestLockWaitAndCancellation(t *testing.T) {
	t.Parallel()
	path := runner.LockPath(t.TempDir(), "lint")
	cmd, _ := startRunnerChild(t, "TestLockChild", []string{
		"GOLAW_LOCK_CHILD=" + path,
		"GOLAW_LOCK_DELAY=10s",
	})
	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	defer cancel()
	_, err := runner.Acquire(ctx, path, runner.DemandWait)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancelled wait = %v", err)
	}
	stopRunnerChild(t, cmd)
	lock, err := runner.Acquire(t.Context(), path, runner.DemandWait)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestLockPolicyAndStaleMetadata(t *testing.T) {
	t.Parallel()
	if runner.RungWait != 5*time.Second ||
		runner.DemandWait != 30*time.Minute {
		t.Fatal("wait policy changed")
	}
	dir := t.TempDir()
	path := runner.LockPath(dir, "lint")
	if path != filepath.Join(dir, "golaw", "locks", "lint.lock") {
		t.Fatalf("path = %q", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".pid", []byte("not-a-pid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lock, err := runner.Acquire(t.Context(), path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if releaseErr := lock.Release(); releaseErr != nil {
		t.Fatal(releaseErr)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("stable lock file disappeared: %v", statErr)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = runner.Acquire(ctx, path, runner.DemandWait)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled acquire = %v", err)
	}
	_, err = runner.Acquire(t.Context(), dir, 0)
	if err == nil || errors.Is(err, runner.ErrLocked) {
		t.Fatalf("non-file lock = %v", err)
	}
}

func TestLockUnknownHolderMetadata(t *testing.T) {
	t.Parallel()
	path := runner.LockPath(t.TempDir(), "metadata")
	lock, err := runner.Acquire(t.Context(), path, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lock.Release(); err != nil {
			t.Error(err)
		}
	})
	for _, data := range []string{"", "invalid\n", "-1\n", "+1\n", "0\n", strings.Repeat("9", 100)} {
		if err := os.WriteFile(path+".pid", []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		requireHeldBy(t, path, 0)
	}
	if err := os.Remove(path + ".pid"); err != nil {
		t.Fatal(err)
	}
	requireHeldBy(t, path, 0)
}

func TestLockMetadataFailuresReleaseOwnership(t *testing.T) {
	t.Parallel()
	path := runner.LockPath(t.TempDir(), "failure")
	marker := blockLockPID(t, path)
	if _, err := runner.Acquire(t.Context(), path, 0); err == nil || errors.Is(err, runner.ErrLocked) {
		t.Fatalf("metadata failure = %v", err)
	}
	before, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	lock, err := runner.Acquire(t.Context(), path, 0)
	if err != nil {
		t.Fatal(err)
	}
	blockLockPID(t, path)
	if releaseErr := lock.Release(); releaseErr == nil {
		t.Fatal("release lost metadata removal error")
	}
	if releaseErr := lock.Release(); releaseErr != nil {
		t.Fatal(releaseErr)
	}
	if removeErr := os.Remove(marker); removeErr != nil {
		t.Fatal(removeErr)
	}
	lock, err = runner.Acquire(t.Context(), path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if releaseErr := lock.Release(); releaseErr != nil {
		t.Fatal(releaseErr)
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("lock inode changed: %v", err)
	}
}

func blockLockPID(t *testing.T, path string) string {
	t.Helper()
	pidPath := path + ".pid"
	if err := os.Remove(pidPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	if err := os.MkdirAll(pidPath, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(pidPath, "occupied")
	if err := os.WriteFile(marker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return marker
}

func TestLockCancelledBeforeCreatingPaths(t *testing.T) {
	t.Parallel()
	path := runner.LockPath(t.TempDir(), "cancelled")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := runner.Acquire(ctx, path, 0)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled immediate acquisition = %v", err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled acquisition created a directory: %v", err)
	}
}
