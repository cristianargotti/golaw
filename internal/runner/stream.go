package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
)

// Stream forwards output as it arrives so interactive commands never wait
// for capture completion or lose bytes to the diagnostic bound.
func Stream(ctx context.Context, c Command, stdout, stderr io.Writer) (int, error) {
	if err := ctx.Err(); err != nil {
		return streamResult(ctx, c.Name, nil, err)
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	out, errOut := streamWriters(stdout, stderr, cancel)
	cmd := exec.CommandContext(child, c.Name, c.Args...) //nolint:gosec // The kit names the program and its arguments.
	cmd.Dir, cmd.Env, cmd.Stdin, cmd.WaitDelay = c.Dir, c.Env, c.Stdin, waitDelay
	cmd.Stdout, cmd.Stderr = out.output(), errOut.output()
	if err := cmd.Start(); err != nil {
		return streamResult(ctx, c.Name, nil, err)
	}
	runErr := cmd.Wait()
	writeErr := out.err
	if errOut != out {
		writeErr = errors.Join(writeErr, errOut.err)
	}
	return streamResult(ctx, c.Name, writeErr, runErr)
}

// streamWriter stops a child that could otherwise keep running after its
// consumer failed; Wait can hide that failure behind the child's exit.
type streamWriter struct {
	destination io.Writer
	cancel      context.CancelFunc
	err         error
}

func (w *streamWriter) Write(data []byte) (int, error) {
	n, err := w.destination.Write(data)
	if n != len(data) && err == nil {
		err = io.ErrShortWrite
	}
	if err != nil {
		w.err = err
		w.cancel()
		return n, fmt.Errorf("write process output: %w", err)
	}
	return n, nil
}

// Native handles preserve terminals and keep blocked pipe writes in the child,
// where cancellation can stop them without waiting for a copying goroutine.
func (w *streamWriter) output() io.Writer {
	if w.destination == nil {
		return nil
	}
	if file, ok := w.destination.(*os.File); ok {
		return file
	}
	return w
}

func streamWriters(stdout, stderr io.Writer, cancel context.CancelFunc) (*streamWriter, *streamWriter) {
	out := &streamWriter{destination: stdout, cancel: cancel}
	// Keep exec's single-copy behavior when both streams share a writer.
	if reflect.ValueOf(stdout).Comparable() && stdout == stderr {
		return out, out
	}
	return out, &streamWriter{destination: stderr, cancel: cancel}
}

func streamResult(ctx context.Context, name string, writeErr, runErr error) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, fmt.Errorf("stream %s: %w", name, errors.Join(err, writeErr))
	}
	if writeErr != nil {
		return 0, fmt.Errorf("write %s output: %w", name, writeErr)
	}
	if runErr == nil || errors.Is(runErr, exec.ErrWaitDelay) {
		return 0, nil
	}
	if exit, exited := errors.AsType[*exec.ExitError](runErr); exited {
		return exit.ExitCode(), nil
	}
	return 0, fmt.Errorf("%w: %s: %w", ErrStart, name, runErr)
}
