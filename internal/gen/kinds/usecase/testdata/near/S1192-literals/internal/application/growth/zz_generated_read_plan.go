package growth

import "fmt"

const actionReadPlan = "growth: read plan"

// ReadPlan wraps every error with its action text.
type ReadPlan struct{}

func (u *ReadPlan) wrap(err error) error { return fmt.Errorf("%s: %w", actionReadPlan, err) }

func (u *ReadPlan) first(err error) error { return u.wrap(err) }

func (u *ReadPlan) second(err error) error { return u.wrap(err) }

func (u *ReadPlan) third(err error) error { return u.wrap(err) }
