package runner_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

func TestReadOutputKeepsTheCompleteStream(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int
		code int
	}{
		{"near-below", runner.MaxOutputBytes - 1, 0},
		{"near-exact", runner.MaxOutputBytes, 0},
		{"red-above", runner.MaxOutputBytes + 37, 0},
		{"red-exit", 17, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			runnertest.Install(t, dir, "go", runnertest.Behavior{
				Probe:  []string{"version"},
				Stdout: strings.Repeat("x", tc.size),
				Stderr: strings.Repeat("e", runner.MaxOutputBytes+1),
				Code:   tc.code,
			})
			t.Setenv("PATH", dir)
			var size int64
			read := func(r io.Reader) error {
				var err error
				size, err = io.Copy(io.Discard, r)
				return consumeError(err)
			}
			out, err := runner.ReadOutput(
				t.Context(), runner.Command{Name: "go"}, read,
			)
			if err != nil || size != int64(tc.size) || out.Code != tc.code {
				t.Fatalf("size=%d output=%+v error=%v", size, out, err)
			}
			if out.Stdout != nil || len(out.Stderr) != runner.MaxOutputBytes {
				t.Fatalf("retained stdout=%d stderr=%d",
					len(out.Stdout), len(out.Stderr))
			}
		})
	}
}

func TestReadOutputPreservesConsumerFailure(t *testing.T) {
	dir := t.TempDir()
	runnertest.Install(t, dir, "go", runnertest.Behavior{
		Probe:  []string{"version"},
		Stdout: strings.Repeat("x", runner.MaxOutputBytes+1),
	})
	t.Setenv("PATH", dir)
	read := func(io.Reader) error { return io.ErrClosedPipe }
	_, err := runner.ReadOutput(
		t.Context(), runner.Command{Name: "go"}, read,
	)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("error = %v", err)
	}
}

func TestReadOutputPreservesStartFailure(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	read := func(io.Reader) error {
		t.Fatal("consumer called before start")
		return nil
	}
	_, err := runner.ReadOutput(
		t.Context(), runner.Command{Name: "go"}, read,
	)
	if !errors.Is(err, runner.ErrStart) {
		t.Fatalf("error = %v", err)
	}
}

func TestReadOutputPreservesCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	read := func(io.Reader) error {
		t.Fatal("consumer called after cancellation")
		return nil
	}
	_, err := runner.ReadOutput(ctx, runner.Command{Name: "git"}, read)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func TestReadOutputDrainsAfterTheConsumerReturns(t *testing.T) {
	dir := t.TempDir()
	runnertest.Install(t, dir, "go", runnertest.Behavior{
		Probe: []string{"version"}, Stdout: strings.Repeat("x", runner.MaxOutputBytes+1),
		Stderr: "drained", Code: 7,
	})
	t.Setenv("PATH", dir)
	read := func(io.Reader) error { return nil }
	out, err := runner.ReadOutput(t.Context(), runner.Command{Name: "go"}, read)
	if err != nil || out.Code != 7 || string(out.Stderr) != "drained" || out.Stdout != nil {
		t.Fatalf("drained output = %+v, %v", out, err)
	}
}

func TestReadOutputPassesTheCommandInput(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Write("value.txt", "golaw\n")
	want := repo.Git("hash-object", "value.txt")
	var stdout bytes.Buffer
	read := func(r io.Reader) error {
		_, err := io.Copy(&stdout, r)
		return consumeError(err)
	}
	out, err := runner.ReadOutput(t.Context(), git(repo, "", "hash-object", "--stdin"), read)
	if err != nil || out.Code != 0 || stdout.String() != "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391\n" {
		t.Fatalf("empty stdin output = %+v, %q, %v", out, stdout.String(), err)
	}
	stdout.Reset()
	out, err = runner.ReadOutput(t.Context(), git(repo, "golaw\n", "hash-object", "--stdin"), read)
	if err != nil || out.Code != 0 || strings.TrimSpace(stdout.String()) != want {
		t.Fatalf("nonempty stdin output = %+v, %q, %v", out, stdout.String(), err)
	}
}

func TestReadOutputBoundsInheritedStderrWait(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		script string
		code   int
	}{
		"success": {"sleep 10 >&2 & echo $!", 0},
		"failure": {"sleep 10 >&2 & echo $!; exit 3", 3},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var stdout bytes.Buffer
			read := func(r io.Reader) error {
				_, err := io.Copy(&stdout, r)
				return consumeError(err)
			}
			start := time.Now()
			out, err := runner.ReadOutput(t.Context(), shell(t, tc.script), read)
			took := time.Since(start)
			reap(t, stdout.Bytes())
			if err != nil || out.Code != tc.code || took > 3*time.Second {
				t.Fatalf("inherited stderr output = %+v, %v, delay = %s", out, err, took)
			}
		})
	}
}

func TestReadOutputCancellationUnblocksTheConsumer(t *testing.T) {
	t.Parallel()
	for name, script := range map[string]string{
		"process":    "exec sleep 10",
		"descendant": "sleep 10 & echo $!; exec sleep 10",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
			defer cancel()
			var stdout bytes.Buffer
			read := func(r io.Reader) error {
				_, err := io.Copy(&stdout, r)
				return consumeError(err)
			}
			_, err := runner.ReadOutput(ctx, shell(t, script), read)
			deadline, _ := ctx.Deadline()
			late := time.Since(deadline)
			reap(t, stdout.Bytes())
			if !errors.Is(err, context.DeadlineExceeded) || late > 2*time.Second {
				t.Fatalf("canceled stream = %v, delay = %s", err, late)
			}
		})
	}
}

func TestReadOutputCancelsAfterConsumerFailure(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		cancel bool
		want   error
	}{
		"consumer": {false, io.ErrClosedPipe},
		"caller":   {true, context.Canceled},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			read := func(r io.Reader) error {
				if _, err := io.CopyN(io.Discard, r, 6); err != nil {
					return consumeError(err)
				}
				if tc.cancel {
					cancel()
				}
				return io.ErrClosedPipe
			}
			start := time.Now()
			_, err := runner.ReadOutput(ctx, shell(t, "echo ready; exec sleep 10"), read)
			if !errors.Is(err, tc.want) || time.Since(start) > 2*time.Second {
				t.Fatalf("consumer failure = %v, want %v", err, tc.want)
			}
		})
	}
}

func consumeError(err error) error {
	if err != nil {
		return fmt.Errorf("consume stdout: %w", err)
	}
	return nil
}
