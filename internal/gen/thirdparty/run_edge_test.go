package thirdparty_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/thirdparty"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// A generator that prints a long failure is quoted to a bound with the
// rest counted, so a reason stays readable.
func TestExecBoundsTheQuotedOutput(t *testing.T) {
	t.Parallel()
	var long strings.Builder
	for i := range 25 {
		fmt.Fprintf(&long, "plans.sql:%d: syntax error\n", i+1)
	}
	broken := runnertest.Install(t, t.TempDir(), "sqlc", runnertest.Behavior{Stderr: long.String(), Code: 1})
	_, err := thirdparty.Exec(t.Context(), broken, t.TempDir(), sqlc, nil)
	if !errors.Is(err, thirdparty.ErrFailed) || !strings.Contains(err.Error(), "plans.sql:20:") ||
		strings.Contains(err.Error(), "plans.sql:21:") || !strings.Contains(err.Error(), "... and 5 more lines") {
		t.Errorf("a long failure = %v", err)
	}
}

// A run its context ends is neither missing nor failed: the caller sees
// the context's error.
func TestExecAnswersTheContextError(t *testing.T) {
	t.Parallel()
	fake := runnertest.Install(t, t.TempDir(), "sqlc", runnertest.Behavior{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := thirdparty.Exec(ctx, fake, t.TempDir(), sqlc, nil)
	if !errors.Is(err, context.Canceled) || errors.Is(err, thirdparty.ErrMissing) || errors.Is(err, thirdparty.ErrFailed) {
		t.Errorf("a canceled run = %v", err)
	}
}
