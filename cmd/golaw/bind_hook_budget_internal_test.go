package main

import (
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/toolchain"
)

// The external work of one post event shares one deadline (A28): the
// regeneration's generators, the mechanical formatter and the M6 linters
// each take only what the stages before them left.
func TestTheStagesOfOneEventShareOneDeadline(t *testing.T) {
	t.Parallel()
	budget := &eventBudget{}
	first, cancel := budget.context(t.Context())
	defer cancel()
	deadline, ok := first.Deadline()
	if !ok || time.Until(deadline) > toolchain.HookEventBudget {
		t.Fatalf("the first stage deadline = %v, %v", deadline, ok)
	}
	later, cancelLater := budget.context(t.Context())
	defer cancelLater()
	if again, _ := later.Deadline(); !again.Equal(deadline) {
		t.Errorf("a later stage got a fresh deadline %v, want the event's %v", again, deadline)
	}
	budget.reset()
	next, cancelNext := budget.context(t.Context())
	defer cancelNext()
	if fresh, _ := next.Deadline(); fresh.Before(deadline) {
		t.Errorf("the next event kept the spent deadline %v", fresh)
	}
}

// A spent event starts no stage: the context of every later stage is done.
func TestASpentEventLeavesNoTimeToALaterStage(t *testing.T) {
	t.Parallel()
	budget := &eventBudget{deadline: time.Now().Add(-time.Millisecond)}
	ctx, cancel := budget.context(t.Context())
	defer cancel()
	if ctx.Err() == nil {
		t.Error("a stage after the event's budget got time to run")
	}
	var none *eventBudget
	alone, cancelAlone := none.context(t.Context())
	defer cancelAlone()
	if _, ok := alone.Deadline(); !ok || alone.Err() != nil {
		t.Error("a binding without a budget runs its stage unbounded or spent")
	}
	none.reset()
}
