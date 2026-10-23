// Package paths holds the checks that judge a path by its name: what may
// live in the tree, what an ignore file may hide, and what a hook may read.
// The secret name list of section 8.4 of the specification lives here as
// data, shared by the tree rule and the read rule.
package paths

import (
	"github.com/cristianargotti/golaw/internal/model"
)

// rule is the rule reference of the family until docs/rules.md ships.
const rule = "rules/4.2@1"

// Checks lists every check of the family in report order.
func Checks() []model.Check {
	return []model.Check{Forbidden(), GeneratedTracked(), SecretMaterial(), ProfileForbidden()}
}

// spec builds the specification every paths check shares: the core owner,
// the family rule and the proof key equal to the id.
func spec(id string, scope model.Scope) model.Spec {
	return model.Spec{ID: id, Rule: rule, Scope: scope, Owner: model.OwnerCore, Proof: id}
}

// deny builds a finding at a path; the engine fills the check, the rule and
// the fingerprint.
func deny(path string, line int, code, message string) model.Finding {
	return model.Finding{Path: path, Line: line, Code: code, Message: message, Severity: model.SeverityDeny}
}
