package policyfiles

import "github.com/cristianargotti/golaw/internal/model"

// buildChecks are the checks of the build half of the family, in report
// order: policy.makefile, policy.ci-target, policy.generated-fresh and
// policy.hook-tests.
func buildChecks() []model.Check {
	return []model.Check{Makefile(), CITarget(), GeneratedFresh()}
}
