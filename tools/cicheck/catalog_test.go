package cicheck_test

import "testing"

func auditRules(t *testing.T) []auditRule {
	t.Helper()
	return []auditRule{publicTextRule(), skillRule()}
}
