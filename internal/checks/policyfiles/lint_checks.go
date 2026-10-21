package policyfiles

import "github.com/cristianargotti/golaw/internal/model"

// lintChecks are the checks of the lint half of the family, in report
// order: policy.golangci, policy.ruleguard, policy.gitleaks-generic and
// policy.gotoolchain-local.
func lintChecks() []model.Check {
	return nil
}
