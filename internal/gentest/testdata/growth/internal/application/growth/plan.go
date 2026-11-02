package growth

import "time"

// AccountID names one account.
type AccountID string

// Plan is the plan of one account.
type Plan struct {
	Account  AccountID
	Name     string
	Archived bool
	Since    time.Time
}
