package policyfiles

import (
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
)

// Checks lists every check of the family in report order: the lint half,
// the build half, the dependency half and the checks the profile data
// turns on, each listed in a file of its own so the halves grow apart.
func Checks() []model.Check {
	return slices.Concat(lintChecks(), buildChecks(), depsChecks(), profileChecks())
}
