package ledgertest

import (
	"context"
	"fmt"
)

const action = "ledger fake answers: %w"

// Fake answers every method with the same error text.
type Fake struct{}

// ReadPlan fails with the action text.
func (f *Fake) ReadPlan(ctx context.Context) error {
	return fmt.Errorf(action, ctx.Err())
}

// ArchivePlan fails with the action text.
func (f *Fake) ArchivePlan(ctx context.Context) error {
	return fmt.Errorf(action, ctx.Err())
}

// PurgePlan fails with the action text.
func (f *Fake) PurgePlan(ctx context.Context) error {
	return fmt.Errorf(action, ctx.Err())
}
