package hook_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// slowReader hands its bytes over only after the budget has passed, so
// the answer is late whatever it says.
type slowReader struct {
	delay time.Duration
	body  io.Reader
}

func (reader slowReader) Read(p []byte) (int, error) {
	time.Sleep(reader.delay)
	return reader.body.Read(p) //nolint:wrapcheck // io.EOF must reach the caller unwrapped
}

func TestRunDeadlineCoversALateInput(t *testing.T) {
	t.Parallel()
	adapter, dependencies := harness(t, request("safe"))
	dependencies.Budget = time.Millisecond
	reader := slowReader{delay: 20 * time.Millisecond, body: strings.NewReader("{}")}
	reply := hook.Run(t.Context(), "claude", reader, adapter, dependencies)
	if reply.ExitCode != 2 || !strings.Contains(reply.Stderr, "deadline") {
		t.Fatal(reply)
	}
}

func TestRunDeadlineAndPanic(t *testing.T) {
	t.Parallel()
	adapter, dependencies := harness(t, request("safe"))
	dependencies.Budget = time.Millisecond
	dependencies.Snapshot = func(ctx context.Context, _ hook.Repository) (model.Snapshot, error) {
		<-ctx.Done()
		return nil, fmt.Errorf("snapshot: %w", ctx.Err())
	}
	reply := hook.Run(t.Context(), "claude", strings.NewReader("{}"), adapter, dependencies)
	if reply.ExitCode != 2 || !strings.Contains(reply.Stderr, "deadline") {
		t.Fatal(reply)
	}
	dependencies.Budget = time.Second
	dependencies.Snapshot = func(context.Context, hook.Repository) (model.Snapshot, error) {
		panic("controlled failure")
	}
	reply = hook.Run(t.Context(), "claude", strings.NewReader("{}"), adapter, dependencies)
	if reply.ExitCode != 2 || !strings.Contains(reply.Stderr, "panicked") {
		t.Fatal(reply)
	}
}

func TestRunSuccessMalformedAndBudget(t *testing.T) {
	t.Parallel()
	adapter, dependencies := harness(t, request("safe"))
	reply := hook.Run(t.Context(), "claude", strings.NewReader("{}"), adapter, dependencies)
	if reply.ExitCode != 0 || string(reply.Stdout) != "allow" {
		t.Fatal(reply)
	}
	reply = hook.Run(t.Context(), "claude", strings.NewReader("{"), adapter, dependencies)
	if reply.ExitCode != 2 || reply.Stderr == "" {
		t.Fatal(reply)
	}
	dependencies.Budget = 10 * time.Second
	reply = hook.Run(t.Context(), "claude", strings.NewReader("{}"), adapter, dependencies)
	if reply.ExitCode != 2 {
		t.Fatal("host-sized internal deadline accepted")
	}
}
