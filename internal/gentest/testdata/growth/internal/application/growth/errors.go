package growth

import "errors"

// ErrPlanNotFound is the answer of a port for an account without a plan.
var ErrPlanNotFound = errors.New("growth: plan not found")
