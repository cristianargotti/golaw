package policyfiles

import (
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
)

// Checks lists every check of the family in report order: the lint half,
// the build half and the dependency half, each listed in a file of its own
// so the three halves grow apart.
func Checks() []model.Check {
	return slices.Concat(lintChecks(), buildChecks(), depsChecks())
}
