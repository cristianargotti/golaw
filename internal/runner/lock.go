package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RungWait keeps an armed rung from waiting beyond seconds.
const RungWait = 5 * time.Second

// DemandWait lets an explicitly requested command wait for a busy tool.
const DemandWait = 30 * time.Minute

const (
	lockRetry   = 20 * time.Millisecond
	lockDirMode = 0o700
	lockMode    = 0o600
)

// Lock retains the handle so ownership ends when its process dies.
type Lock struct {
	state lockState
}

// LockedError preserves the holder's identity for an actionable diagnostic.
type LockedError struct {
	Path string
	PID  int
}

func (e *LockedError) Error() string {
	return fmt.Sprintf("lock %s is held by process %d", e.Path, e.PID)
}

func (e *LockedError) Unwrap() error { return ErrLocked }

// LockPath isolates each worktree's commands from other worktrees.
func LockPath(gitDir, name string) string {
	return filepath.Join(gitDir, "golaw", "locks", name+".lock")
}

// Acquire polls under ctx because a blocking OS call cannot honor cancellation.
func Acquire(ctx context.Context, path string, wait time.Duration) (*Lock, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := openLock(path)
	if err != nil {
		return nil, err
	}
	lock := &Lock{state: lockState{file: file}}
	if err := lock.acquire(ctx, wait); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if err := writeLockPID(path); err != nil {
		return nil, errors.Join(err, lock.Release())
	}
	return lock, nil
}

// Release keeps the inode in place so old and new waiters share ownership.
func (l *Lock) Release() error {
	if l.state.file == nil {
		return nil
	}
	pidErr := removeLockPID(l.state.file.Name())
	unlockErr := l.unlock()
	closeErr := l.state.file.Close()
	l.state.file = nil
	return errors.Join(pidErr, unlockErr, closeErr)
}

func (l *Lock) acquire(ctx context.Context, wait time.Duration) error {
	deadline, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := l.tryLock(); !errors.Is(err, ErrLocked) {
			return err
		}
		if err := l.retry(ctx, deadline); err != nil {
			return err
		}
	}
}

func (l *Lock) retry(ctx, deadline context.Context) error {
	timer := time.NewTimer(lockRetry)
	defer timer.Stop()
	select {
	case <-deadline.Done():
		if err := ctx.Err(); err != nil {
			return err
		}
		path := l.state.file.Name()
		return &LockedError{Path: path, PID: readLockPID(path)}
	case <-timer.C:
		return nil
	}
}

func openLock(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), lockDirMode); err != nil {
		return nil, fmt.Errorf("create lock directory %s: %w", path, err)
	}
	if err := regularLock(path, nil); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	file, err := os.OpenFile(path, lockOpenFlags(), lockMode) //nolint:gosec // The lock path is checked before and after opening without truncation.
	if err != nil {
		return nil, fmt.Errorf("open lock %s: %w", path, err)
	}
	info, err := file.Stat()
	if err == nil {
		err = regularLock(path, info)
	}
	if err != nil {
		return nil, errors.Join(fmt.Errorf("inspect lock %s: %w", path, err), file.Close())
	}
	return file, nil
}

func regularLock(path string, opened os.FileInfo) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect lock path %s: %w", path, err)
	}
	if !info.Mode().IsRegular() || (opened != nil && !os.SameFile(info, opened)) {
		return fmt.Errorf("lock path %s is not a stable regular file: %w", path, os.ErrInvalid)
	}
	return nil
}
