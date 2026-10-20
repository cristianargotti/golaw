package runner_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
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
