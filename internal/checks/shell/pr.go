package shell

import (
	"github.com/cristianargotti/golaw/internal/model"
)

// PullRequest is shell.pr: gh pr create and gh repo create are denied when
// the policy says so (deny_pr) and reported as a note otherwise.
func PullRequest() model.Check {
	return commandRule{spec: hookSpec("shell.pr"), judge: judgePullRequest}
}

func judgePullRequest(in model.Input, segs []segment) ([]model.Finding, error) {
	for _, seg := range segs {
		words := unwrap(seg)
		if len(words) < 3 || base(words[0]) != "gh" || words[2] != "create" || (words[1] != "pr" && words[1] != "repo") {
			continue
		}
		return []model.Finding{optional(in.Policy.Shell.DenyPR, "pr",
			"gh "+words[1]+" create is denied by the policy; the person opens it",
			"gh "+words[1]+" create is allowed by the policy (deny_pr is off); the skill asks the person first")}, nil
	}
	return nil, nil
}
