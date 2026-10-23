package policyfiles

import (
	"context"
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
)

// Checks lists every check of the family in report order: the lint half,
// the build half and the dependency half, each listed in a file of its own
// so the three halves grow apart. Each one has the snapshot load the
// family's configuration before it runs, so the first check of a snapshot
// loads it in one question whether it reads a file or selects a tree.
func Checks() []model.Check {
	checks := slices.Concat(lintChecks(), buildChecks(), depsChecks())
	for i, check := range checks {
		checks[i] = prefetching{check}
	}
	return checks
}

// prefetching is a check of the family that prefetches the configuration
// before it runs; a snapshot frozen from git loads it once, the first time.
type prefetching struct{ model.Check }

// Run prefetches and runs the check.
func (c prefetching) Run(ctx context.Context, in model.Input) (model.Result, error) {
	prefetch(in)
	return c.Check.Run(ctx, in) //nolint:wrapcheck // the check names its own error
}
