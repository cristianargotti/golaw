package runner_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

func TestStreamCancellationStopsBlockedNativeOutput(t *testing.T) {
	t.Parallel()
	const deadline = 500 * time.Millisecond
	const pipeWait, margin = time.Second, time.Second
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	_, writer := streamPipe(t)
	command := runner.Command{
		Name: program, Args: []string{"-test.run=^TestStreamChild$"}, Dir: t.TempDir(),
		Env: append(gittest.Isolated(), "GOLAW_STREAM_CHILD=1"),
	}
	started := time.Now()
	ctx, cancel := context.WithTimeout(t.Context(), deadline)
	defer cancel()
	// The child writes over 4 MiB while the output pipe stays unread.
	_, err = runner.Stream(ctx, command, writer, io.Discard)
	elapsed := time.Since(started)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocked output deadline = %v", err)
	}
	if elapsed > deadline+pipeWait+margin {
		t.Fatalf("blocked output returned after %s, want at most %s",
			elapsed, deadline+pipeWait+margin)
	}
}

// pidOutput cancels on the first write, the pid the shell prints once the
// descendant that holds the pipes has started.
type pidOutput struct {
	output   bytes.Buffer
	cancel   context.CancelFunc
	canceled time.Time
}

func (w *pidOutput) Write(data []byte) (int, error) {
	n, err := w.output.Write(data)
	if err != nil {
		return n, fmt.Errorf("keep the shell output: %w", err)
	}
	if w.canceled.IsZero() {
		w.canceled = time.Now()
		w.cancel()
	}
	return n, nil
}

// A writer that is no file makes exec copy through a pipe the descendant
// keeps open, so only the bounded wait for the pipes lets Stream return.
func TestStreamCancellationBoundsTheWaitForHeldPipes(t *testing.T) {
	t.Parallel()
	const pipeWait, margin = time.Second, 2 * time.Second
	command := shell(t, "sleep 30 & echo $!; exec sleep 30")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	output := &pidOutput{cancel: cancel}
	_, err := runner.Stream(ctx, command, output, io.Discard)
	returned := time.Now()
	reap(t, output.output.Bytes())
	if !errors.Is(err, context.Canceled) || output.canceled.IsZero() {
		t.Fatalf("held pipes = %v, canceled at %v", err, output.canceled)
	}
	if late := returned.Sub(output.canceled); late > pipeWait+margin {
		t.Fatalf("Stream answered %s after the cancellation, want at most %s",
			late, pipeWait+margin)
	}
}

// A shell that kills itself leaves no exit status to pass through, so the
// error has to name the signal.
func TestStreamNamesTheSignalThatEndedTheChild(t *testing.T) {
	t.Parallel()
	code, err := runner.Stream(t.Context(), shell(t, "kill -9 $$"), io.Discard, io.Discard)
	var exit *exec.ExitError
	if code != 0 || !errors.As(err, &exit) ||
		!strings.Contains(err.Error(), "sh ended by signal: killed") {
		t.Fatalf("signal death = %d, %v", code, err)
	}
}

func TestStreamPreservesNativeFileOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	program := runnertest.Install(t, dir, "stream", runnertest.Behavior{
		Probe: []string{"version"}, Stdout: "result\n", Stderr: "detail\n",
	})
	path := filepath.Join(dir, "output.txt")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	code, err := runner.Stream(t.Context(), runner.Command{
		Name: program, Dir: dir, Env: gittest.Isolated(),
	}, file, file)
	if err != nil || code != 0 {
		t.Fatalf("native output = %d, %v", code, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "result\ndetail\n" {
		t.Fatalf("native file = %q, %v", data, err)
	}
}
