package runner_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

type streamObserver struct {
	mu     sync.Mutex
	counts [2]int
	fired  bool
	finish func() error
}

type observedStream struct {
	observer *streamObserver
	index    int
}

func (w observedStream) Write(data []byte) (int, error) {
	o := w.observer
	o.mu.Lock()
	defer o.mu.Unlock()
	o.counts[w.index] += len(data)
	if !o.fired && o.counts[0] > runner.MaxOutputBytes &&
		o.counts[1] > runner.MaxOutputBytes {
		o.fired = true
		if err := o.finish(); err != nil {
			return 0, err
		}
	}
	return len(data), nil
}

func TestStreamChild(t *testing.T) {
	t.Parallel()
	if os.Getenv("GOLAW_STREAM_CHILD") != "1" {
		return
	}
	data := strings.Repeat("x", runner.MaxOutputBytes+1)
	for _, out := range []io.Writer{os.Stdout, os.Stderr} {
		if _, err := io.WriteString(out, data); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := io.ReadFull(os.Stdin, make([]byte, 1)); err != nil {
		t.Fatal(err)
	}
}

func TestStreamWritesBeforeExitWithoutTheCaptureBound(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		cancel bool
	}{
		{"near-streams-before-exit", false},
		{"red-cancel-running-child", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			program, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			input, supply := streamPipe(t)
			observer := &streamObserver{finish: streamFinish(tc.cancel, cancel, supply)}
			code, err := runner.Stream(ctx, runner.Command{
				Name: program,
				Args: []string{"-test.run=^TestStreamChild$"},
				Dir:  t.TempDir(),
				Env: append(
					gittest.Isolated(), "GOLAW_STREAM_CHILD=1",
				),
				Stdin: input,
			}, observedStream{observer: observer, index: 0},
				observedStream{observer: observer, index: 1})
			requireStreamEnd(t, tc.cancel, code, err)
			requireStreamFired(t, observer)
		})
	}
}

// streamPipe is the pipe the child reads its one byte from, closed when
// the test ends.
func streamPipe(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	input, supply, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := errors.Join(input.Close(), supply.Close()); err != nil {
			t.Error(err)
		}
	})
	return input, supply
}

// streamFinish is what the observer does once both streams passed the
// capture bound: cancel the run, or supply the byte the child waits for.
func streamFinish(canceled bool, cancel context.CancelFunc, supply *os.File) func() error {
	return func() error {
		if canceled {
			cancel()
			return nil
		}
		if _, err := supply.Write([]byte{'x'}); err != nil {
			return fmt.Errorf("supply the child: %w", err)
		}
		return nil
	}
}

// requireStreamEnd requires a canceled run to answer the cancellation and
// any other run to exit 0.
func requireStreamEnd(t *testing.T, canceled bool, code int, err error) {
	t.Helper()
	if canceled {
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled stream = %v", err)
		}
		return
	}
	if err != nil || code != 0 {
		t.Fatalf("stream = %d, %v", code, err)
	}
}

// requireStreamFired requires both streams past the capture bound when
// the observer fired.
func requireStreamFired(t *testing.T, observer *streamObserver) {
	t.Helper()
	observer.mu.Lock()
	counts, fired := observer.counts, observer.fired
	observer.mu.Unlock()
	if !fired || counts[0] <= runner.MaxOutputBytes ||
		counts[1] <= runner.MaxOutputBytes {
		t.Fatalf("streams did not trigger completion: %v, %v",
			counts, fired)
	}
}

type closedStream struct{}

func (closedStream) Write([]byte) (int, error) {
	return 0, os.ErrClosed
}

func TestStreamStatusStartAndWriterFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		code int
	}{
		{"near-success", 0},
		{"red-child-status", 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			program := runnertest.Install(t, dir, "stream", runnertest.Behavior{
				Probe:  []string{"version"},
				Stdout: "result\n", Stderr: "detail\n", Code: tc.code,
			})
			var stdout, stderr bytes.Buffer
			code, err := runner.Stream(t.Context(), runner.Command{
				Name: program, Dir: dir, Env: gittest.Isolated(),
			}, &stdout, &stderr)
			if err != nil || code != tc.code ||
				stdout.String() != "result\n" || stderr.String() != "detail\n" {
				t.Fatalf("stream = %d, %v, %q, %q",
					code, err, stdout.String(), stderr.String())
			}
		})
	}
	t.Run("red-start", func(t *testing.T) {
		t.Parallel()
		program := filepath.Join(t.TempDir(), "missing-program")
		_, err := runner.Stream(t.Context(), runner.Command{
			Name: program,
		}, io.Discard, io.Discard)
		if !errors.Is(err, runner.ErrStart) ||
			!strings.Contains(err.Error(), program) {
			t.Fatalf("start = %v", err)
		}
	})
	t.Run("red-writer", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		program := runnertest.Install(t, dir, "stream", runnertest.Behavior{
			Probe:  []string{"version"},
			Stdout: "result\n",
		})
		_, err := runner.Stream(t.Context(), runner.Command{
			Name: program, Dir: dir, Env: gittest.Isolated(),
		}, closedStream{}, io.Discard)
		if !errors.Is(err, os.ErrClosed) {
			t.Fatalf("writer = %v", err)
		}
	})
}

type shortStream struct{}

func (shortStream) Write([]byte) (int, error) { return 0, nil }

func TestStreamStopsTheChildAfterWriterFailure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		writer io.Writer
		want   error
	}{
		{"closed", closedStream{}, os.ErrClosed},
		{"short", shortStream{}, io.ErrShortWrite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			program, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			input, _ := streamPipe(t)
			_, err = runner.Stream(ctx, runner.Command{
				Name: program, Args: []string{"-test.run=^TestStreamChild$"},
				Dir: t.TempDir(), Stdin: input,
				Env: append(gittest.Isolated(), "GOLAW_STREAM_CHILD=1"),
			}, tc.writer, io.Discard)
			if !errors.Is(err, tc.want) || ctx.Err() != nil {
				t.Fatalf("writer failure = %v, context = %v", err, ctx.Err())
			}
		})
	}
}

func TestStreamPreservesSharedAndNilWriters(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	program := runnertest.Install(t, dir, "stream", runnertest.Behavior{
		Probe: []string{"version"}, Stdout: "result\n", Stderr: "detail\n",
	})
	c := runner.Command{Name: program, Dir: dir, Env: gittest.Isolated()}
	var output bytes.Buffer
	code, err := runner.Stream(t.Context(), c, &output, &output)
	if err != nil || code != 0 ||
		!strings.Contains(output.String(), "result\n") ||
		!strings.Contains(output.String(), "detail\n") {
		t.Fatalf("shared streams = %d, %v, %q", code, err, output.String())
	}
	code, err = runner.Stream(t.Context(), c, nil, nil)
	if err != nil || code != 0 {
		t.Fatalf("nil streams = %d, %v", code, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = runner.Stream(ctx, c, nil, nil)
	if !errors.Is(err, context.Canceled) || len(runnertest.Calls(t, dir, "stream")) != 2 {
		t.Fatalf("canceled run = %v, calls = %v", err, runnertest.Calls(t, dir, "stream"))
	}
}

type sliceStream []byte

func (sliceStream) Write(data []byte) (int, error) { return len(data), nil }

func TestStreamHandlesUncomparableWriterValues(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	program := runnertest.Install(t, dir, "stream", runnertest.Behavior{
		Probe: []string{"version"}, Stdout: "result\n", Stderr: "detail\n",
	})
	output := struct{ io.Writer }{sliceStream{}}
	code, err := runner.Stream(t.Context(), runner.Command{
		Name: program, Dir: dir, Env: gittest.Isolated(),
	}, output, output)
	if err != nil || code != 0 {
		t.Fatalf("uncomparable streams = %d, %v", code, err)
	}
}
