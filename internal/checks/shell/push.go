package shell

import (
	"github.com/cristianargotti/golaw/internal/model"
)

// Push is shell.push: a git push is denied when the policy says so
// (deny_push) and reported as a note otherwise, so the proof sees the rule
// while the core leaves the push to the person's judgment.
func Push() model.Check {
	return commandRule{spec: hookSpec("shell.push"), judge: judgePush}
}

func judgePush(in model.Input, segs []segment) ([]model.Finding, error) {
	for _, call := range gitCalls(segs) {
		if call.sub == subPush {
			return []model.Finding{optional(in.Policy.Shell.DenyPush, "push",
				"git push is denied by the policy; the person pushes",
				"git push is allowed by the policy (deny_push is off); the skill asks the person first")}, nil
		}
	}
	return nil, nil
}

// optional builds a denial when the policy switch is on and a note
// otherwise.
func optional(denied bool, code, denyMessage, noteMessage string) model.Finding {
	if denied {
		return deny(code, denyMessage)
	}
	return model.Finding{Code: code, Message: noteMessage, Severity: model.SeverityNote}
}
