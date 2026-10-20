// Package commits holds the checks that judge a commit at the commit-msg
// rung: the message git records, read through the cleanup git applies
// after the hook, and the staged changes the rung hands over. A check never
// runs git: it reads the commit, the changes and the snapshot of its input.
package commits

import (
	"context"

	"github.com/cristianargotti/golaw/internal/model"
)

// commitRule is the rule reference the shell family already carries for
// the same laws, so a commit judged at the rung and one judged on the
// command line cite one rule.
const commitRule = "rules/4.12@1"

// noCommit is the reason of a check handed an input without a commit,
// which the engine never dispatches in commit mode.
const noCommit = "the commits family judges a commit"

// Checks lists every check of the family in report order.
func Checks() []model.Check {
	return []model.Check{Conventional(), NoTrailers(), NoDashes(), BodyWidth(), Shape()}
}

// spec builds the specification of a commit check: commit scope, the core
// owner, the commit rule and the proof key equal to the id.
func spec(id string) model.Spec {
	return model.Spec{ID: id, Rule: commitRule, Scope: model.ScopeCommit, Owner: model.OwnerCore, Proof: id}
}

// messageRule is a check over the message git records, handed the comment
// string the message was cleaned with.
type messageRule struct {
	spec  model.Spec
	judge func(message, comment string) []model.Finding
}

func (r messageRule) Spec() model.Spec { return r.spec }

// Run judges the message the commit records.
func (r messageRule) Run(_ context.Context, in model.Input) (model.Result, error) {
	if in.Commit == nil {
		return model.Result{Status: model.NotApplicable(noCommit)}, nil
	}
	findings := r.judge(Clean(*in.Commit), commentOf(*in.Commit))
	return model.Result{Status: model.Measured(), Scope: "1 message", Findings: findings}, nil
}

// deny builds a denial at a line and column of the message; the engine
// fills the check, the rule and the fingerprint.
func deny(code string, line, column int, message string) model.Finding {
	return model.Finding{Code: code, Line: line, Column: column, Message: message, Severity: model.SeverityDeny}
}
