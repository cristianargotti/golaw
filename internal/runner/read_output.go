package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// ReadOutput streams stdout without retaining it, so package metadata is
// complete even when a repository's listing exceeds the diagnostic bound.
func ReadOutput(ctx context.Context, c Command, consume func(io.Reader) error) (Output, error) {
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd, stdout, stderr, err := outputCommand(child, c)
	if err != nil {
		return outputResult(ctx, c.Name, Output{}, nil, err)
	}
	defer stdout.Close()
	if startErr := cmd.Start(); startErr != nil {
		return outputResult(ctx, c.Name, Output{}, nil, startErr)
	}
	readErr := consume(stdout)
	if readErr == nil {
		_, readErr = io.Copy(io.Discard, stdout)
	}
	if readErr != nil {
		cancel()
	}
	if closeErr := stdout.Close(); readErr == nil && !errors.Is(closeErr, os.ErrClosed) {
		readErr = closeErr
	}
	err = cmd.Wait()
	out := Output{Stderr: stderr.buf.Bytes()}
	return outputResult(ctx, c.Name, out, readErr, err)
}

func outputCommand(ctx context.Context, c Command) (*exec.Cmd, io.ReadCloser, *bounded, error) {
	cmd := exec.CommandContext(ctx, c.Name, c.Args...) //nolint:gosec // The kit names the program and its arguments.
	cmd.Dir, cmd.Env, cmd.Stdin, cmd.WaitDelay = c.Dir, c.Env, c.Stdin, waitDelay
	stderr := &bounded{limit: MaxOutputBytes}
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open stdout: %w", err)
	}
	cmd.Cancel = func() error {
		killErr := cmd.Process.Kill()
		// Wait cannot close an inherited stdout until the consumer returns.
		closeErr := stdout.Close()
		if errors.Is(closeErr, os.ErrClosed) {
			closeErr = nil
		}
		if killErr != nil || closeErr != nil {
			return fmt.Errorf("stop process: %w", errors.Join(killErr, closeErr))
		}
		return nil
	}
	return cmd, stdout, stderr, nil
}

func outputResult(ctx context.Context, name string, out Output, readErr, runErr error) (Output, error) {
	if err := ctx.Err(); err != nil {
		return out, fmt.Errorf("run %s: %w", name, err)
	}
	if readErr != nil {
		return out, fmt.Errorf("read %s: %w", name, readErr)
	}
	if runErr == nil || errors.Is(runErr, exec.ErrWaitDelay) {
		return out, nil
	}
	if exit, exited := errors.AsType[*exec.ExitError](runErr); exited {
		out.Code = exit.ExitCode()
		return out, nil
	}
	return out, fmt.Errorf("%w: %s: %w", ErrStart, name, runErr)
}
