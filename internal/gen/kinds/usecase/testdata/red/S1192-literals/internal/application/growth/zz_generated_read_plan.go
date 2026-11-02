package growth

import "fmt"

// ReadPlan wraps every error with its action text.
type ReadPlan struct{}

func (u *ReadPlan) first(err error) error { return fmt.Errorf("growth: read plan: %w", err) }

func (u *ReadPlan) second(err error) error { return fmt.Errorf("growth: read plan: %w", err) }

func (u *ReadPlan) third(err error) error { return fmt.Errorf("growth: read plan: %w", err) }
