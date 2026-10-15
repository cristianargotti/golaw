// Package shell holds the checks that judge a shell command before it runs:
// the hook bypass, the commit message laws a hook can see, the commit shape,
// secret material named on the command line, the commands that print a
// credential, and the optional push and pull request denials. A check never
// runs the command: it reads the words of the command line and the snapshot.
package shell

import (
	"context"

	"github.com/cristianargotti/golaw/internal/model"
)

// commitRule is the rule reference of the commit laws a hook sees, until
// docs/rules.md ships.
const commitRule = "rules/4.12@1"

// Checks lists every check of the family in report order.
func Checks() []model.Check {
	return []model.Check{NoVerify(), Trailers(), Dashes(), BodyWidth(), Shape(), SecretTokens()}
}

// spec builds the specification of a commit law check: event scope, the
// core owner, the commit rule and the proof key equal to the id.
func spec(id string) model.Spec {
	return model.Spec{ID: id, Rule: commitRule, Scope: model.ScopeEvent, Owner: model.OwnerCore, Proof: id}
}

// judge is what one check does with the segments of a command: it returns
// its findings, or an error when the snapshot could not be read.
type judge func(in model.Input, segs []segment) ([]model.Finding, error)

// commandRule is a check over the segments of a shell command.
type commandRule struct {
	spec  model.Spec
	judge judge
}

// Spec returns the check specification.
func (r commandRule) Spec() model.Spec { return r.spec }

// Run judges a shell event and answers NotApplicable on any other shape; a
// read failure is Unmeasured, never clean.
func (r commandRule) Run(_ context.Context, in model.Input) (model.Result, error) {
	if in.Event.Shape != model.ShapeShell {
		return model.Result{Status: model.NotApplicable("the shell family judges shell events only")}, nil
	}
	findings, err := r.judge(in, segments(in.Event.Shell.Command))
	if err != nil {
		return model.Result{Status: model.Unmeasured(err.Error())}, nil
	}
	return model.Result{Status: model.Measured(), Scope: "1 command", Findings: findings}, nil
}

// deny builds a denial without a path; the engine fills the check, the rule
// and the fingerprint.
func deny(code, message string) model.Finding {
	return model.Finding{Code: code, Message: message, Severity: model.SeverityDeny}
}
