package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/source"
)

// locked strips only the hook variables so child git commands respect their cwd.
func (b binding) locked(
	ctx context.Context, dir, name string,
	args []string, input io.Reader, stdout, stderr io.Writer,
) (int, error) {
	if err := validateLock(name, args); err != nil {
		return 0, err
	}
	repo, err := source.Discover(ctx, dir)
	if err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return 0, discoverFailed(dir, err)
	}
	lock, err := demandLock(ctx, runner.LockPath(repo.GitDir, name), runner.DemandWait, stderr)
	if err != nil {
		return 0, err
	}
	var code int
	// Finish cleanup before returning so release errors cannot disappear.
	func() {
		defer func() { err = errors.Join(err, lock.Release()) }()
		code, err = runner.Stream(ctx, runner.Command{
			Name: args[0], Args: args[1:], Dir: repo.Root,
			Env: runner.UserEnv(b.environ()), Stdin: input,
		}, stdout, stderr)
	}()
	if err != nil {
		return code, fmt.Errorf("run locked command: %w", err)
	}
	return code, nil
}

// demandLock names the current holder before a potentially long on-demand wait.
func demandLock(ctx context.Context, path string, wait time.Duration, stderr io.Writer) (*runner.Lock, error) {
	lock, err := runner.Acquire(ctx, path, 0)
	if err == nil {
		return lock, nil
	}
	held, busy := errors.AsType[*runner.LockedError](err)
	if !busy {
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	if _, writeErr := fmt.Fprintln(stderr, lockWaitNotice(held)); writeErr != nil {
		return nil, fmt.Errorf("write lock holder: %w", writeErr)
	}
	lock, err = runner.Acquire(ctx, path, wait)
	if err != nil {
		return nil, lockWaitFailed(err)
	}
	return lock, nil
}

func lockWaitNotice(held *runner.LockedError) string {
	if held.PID == 0 {
		return fmt.Sprintf("aguardando a liberação do bloqueio %s, cujo processo não foi identificado", held.Path)
	}
	return fmt.Sprintf("aguardando o processo %d liberar o bloqueio %s", held.PID, held.Path)
}

func lockWaitFailed(err error) error {
	held, busy := errors.AsType[*runner.LockedError](err)
	if !busy {
		return fmt.Errorf("wait for lock: %w", err)
	}
	holder := "o processo que segura o bloqueio"
	if held.PID != 0 {
		holder = fmt.Sprintf("o processo %d", held.PID)
	}
	return lockWaitError{
		cause: err,
		message: fmt.Sprintf("bloqueio %s ocupado; espere %s terminar e rode o comando de novo",
			held.Path, holder),
	}
}

// lockWaitError retains acquisition causes without rendering an unknown PID.
type lockWaitError struct {
	cause   error
	message string
}

func (e lockWaitError) Error() string { return e.message }

func (e lockWaitError) Unwrap() error { return e.cause }

// validateLock rejects paths before a lock name can reach the filesystem.
func validateLock(name string, args []string) error {
	if !validLockName(name) {
		return fmt.Errorf("%w: lock name must start with an ASCII letter or digit and contain only ASCII letters, digits, dots, underscores or hyphens", model.ErrUsage)
	}
	if len(args) == 0 || args[0] == "" {
		return fmt.Errorf("%w: lock requires a nonempty command after --", model.ErrUsage)
	}
	return nil
}

func validLockName(name string) bool {
	if name == "" || !lockAlphaNumeric(rune(name[0])) {
		return false
	}
	for _, r := range name[1:] {
		if !lockAlphaNumeric(r) && r != '.' && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

func lockAlphaNumeric(r rune) bool {
	return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
}
