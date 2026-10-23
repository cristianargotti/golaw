package policyfiles

import "github.com/cristianargotti/golaw/internal/model"

// profileChecks lists the checks the data of the enrolled profile turns on
// (section 5.4, contract D5), in report order; each one answers section
// before it reads.
func profileChecks() []model.Check {
	return []model.Check{ImageStages()}
}
