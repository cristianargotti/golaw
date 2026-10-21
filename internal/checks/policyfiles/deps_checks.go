package policyfiles

import "github.com/cristianargotti/golaw/internal/model"

// depsChecks are the checks of the dependency half of the family, in
// report order: policy.dependency-records, policy.dependabot-cooldown,
// policy.deps-age-config, policy.image-digest, policy.codeowners-foreign,
// policy.openapi-assertions and policy.squawk-config.
func depsChecks() []model.Check {
	return nil
}
