package shell

import (
	"regexp"
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
)

// hooksPath matches the configuration override that disarms the hooks.
var hooksPath = regexp.MustCompile(`(?i)^core\.hookspath=`)

// NoVerify is shell.no-verify: no command skips the git hooks, neither with
// the flag (--no-verify, and -n on commit) nor with a hooks path override.
func NoVerify() model.Check {
	return commandRule{spec: spec("shell.no-verify"), judge: judgeNoVerify}
}

func judgeNoVerify(_ model.Input, segs []segment) ([]model.Finding, error) {
	for _, call := range gitCalls(segs) {
		bypass := call.sub == subCommit && call.has("--no-verify", "-n") ||
			(call.sub == "merge" || call.sub == "rebase" || call.sub == subPush) && call.has("--no-verify") ||
			slices.ContainsFunc(call.words, hooksPath.MatchString)
		if bypass {
			return []model.Finding{deny("no-verify", "the command skips the git hooks; the hooks are the rule")}, nil
		}
	}
	return nil, nil
}
