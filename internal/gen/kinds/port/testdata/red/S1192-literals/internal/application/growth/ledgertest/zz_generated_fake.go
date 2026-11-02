package ledgertest

import (
	"context"
	"fmt"
)

// Fake answers every method with the same error text.
type Fake struct{}

// ReadPlan fails with the action text.
func (f *Fake) ReadPlan(ctx context.Context) error {
	return fmt.Errorf("ledger fake answers: %w", ctx.Err())
}

// ArchivePlan fails with the action text.
func (f *Fake) ArchivePlan(ctx context.Context) error {
	return fmt.Errorf("ledger fake answers: %w", ctx.Err())
}

// PurgePlan fails with the action text.
func (f *Fake) PurgePlan(ctx context.Context) error {
	return fmt.Errorf("ledger fake answers: %w", ctx.Err())
}
