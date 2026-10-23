package policyfiles

import "github.com/cristianargotti/golaw/internal/model"

// family is the id prefix of every check of the family.
const family = "policy."

// The rule references of the family: the subsection of the inventory each
// check ports, versioned, so a finding's fingerprint never moves with the
// numbering of docs/rules.md.
const (
	ruleLinters  = "rules/4.5@1"
	ruleTests    = "rules/4.6@1"
	ruleSecurity = "rules/4.7@1"
	ruleSupply   = "rules/4.8@1"
	ruleData     = "rules/4.9@1"
	ruleContract = "rules/4.10@1"
	ruleDelivery = "rules/4.12@1"
	ruleLadder   = "rules/4.13@1"
	ruleProfile  = "rules/4.14@1"
)

// spec builds the specification of one check of the family: the id is the
// family plus the name, the scope is the tree, so a check judges the
// worktree or the index and never runs in an agent hook, the owner is core
// and the proof key is the id.
func spec(name, rule string, needs ...string) model.Spec {
	id := family + name
	return model.Spec{ID: id, Rule: rule, Scope: model.ScopeTree, Owner: model.OwnerCore, Needs: needs, Proof: id}
}
