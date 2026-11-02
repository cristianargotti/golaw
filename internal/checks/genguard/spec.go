package genguard

import "github.com/cristianargotti/golaw/internal/model"

// rule is the section of docs/rules.md the whole family answers to.
const rule = "rules/4.17@1"

// hookOnly is why a denial of the family skips every input but an agent's
// write in the hook.
const hookOnly = "an agent's write in the hook only"

// spec builds the specification of a gen check: the core owner, the family
// rule and the proof key equal to the id.
func spec(id string, scope model.Scope) model.Spec {
	return model.Spec{ID: id, Rule: rule, Scope: scope, Owner: model.OwnerCore, Proof: id}
}

// deny builds a denial at a path; the engine fills the check, the rule and
// the fingerprint.
func deny(path string, line int, code, message string) model.Finding {
	return model.Finding{Path: path, Line: line, Code: code, Message: message, Severity: model.SeverityDeny}
}

// agentWrite reports whether the input is an agent's write before it runs,
// the only moment a read only denial is certain: after the write, or in a
// scan, a generator's own output cannot be told from a hand edit.
func agentWrite(in model.Input) bool {
	return in.Mode == model.ModeHook && in.Event != nil && in.Event.Shape == model.ShapeWrite
}
