package main

import (
	"context"
	"time"

	"github.com/cristianargotti/golaw/internal/toolchain"
)

// eventBudget is the budget of the external work of one hook event (A28):
// the first stage that runs a tool starts it, and every later stage of the
// same event, the generators, the formatter and the M6 linters, takes only
// what is left, so the event stops at 3 s however many stages it runs.
type eventBudget struct {
	deadline time.Time
}

// context answers ctx bounded by the event's deadline, started now when no
// stage of the event started it yet; a binding without a budget bounds the
// stage alone.
func (b *eventBudget) context(ctx context.Context) (context.Context, context.CancelFunc) {
	if b == nil {
		return context.WithTimeout(ctx, toolchain.HookEventBudget)
	}
	if b.deadline.IsZero() {
		b.deadline = time.Now().Add(toolchain.HookEventBudget)
	}
	return context.WithDeadline(ctx, b.deadline)
}

// reset clears the deadline at the start of an event.
func (b *eventBudget) reset() {
	if b != nil {
		b.deadline = time.Time{}
	}
}
