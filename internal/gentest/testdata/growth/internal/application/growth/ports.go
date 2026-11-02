// Package growth holds the plans of an account and the ports they need.
package growth

import (
	"context"
	"time"
)

// Ledger stores the plans of an account.
type Ledger interface {
	// ReadPlan reads the plan of one account.
	ReadPlan(ctx context.Context, account AccountID) (Plan, error)
	// ArchivePlan archives the plan of one account.
	ArchivePlan(ctx context.Context, account AccountID) error
}

// Clock tells the time a use case stamps on a plan.
type Clock interface {
	// Now answers the current time.
	Now() time.Time
}
