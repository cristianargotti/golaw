package shell_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestPullRequestIsANoteUnlessDenied(t *testing.T) {
	t.Parallel()
	pr := judge(t, shell.PullRequest(), "gh pr create --fill")
	if c := codes(pr); len(c) != 1 || c[0] != "pr" || pr.Findings[0].Severity != model.SeverityNote {
		t.Errorf("pr under the core policy = %+v", pr.Findings)
	}
	for _, clean := range []string{"gh pr view 1", "gh repo view", "gh", "gh pr", "gh.exe pr view 1"} {
		if c := codes(judge(t, shell.PullRequest(), clean)); len(c) != 0 {
			t.Errorf("%q = %v", clean, c)
		}
	}
	strict := proof.Policy()
	strict.Shell = model.ShellPolicy{DenyPR: true}
	for command, message := range map[string]string{
		"gh repo create mine --private": "gh repo create is denied by the policy; the person opens it",
		"gh.exe pr create --fill":       "gh pr create is denied by the policy; the person opens it",
	} {
		denied, err := shell.PullRequest().Run(t.Context(), input(command, strict, proof.NewTree(nil)))
		if err != nil || len(denied.Findings) != 1 || denied.Findings[0].Severity != model.SeverityDeny || denied.Findings[0].Message != message {
			t.Errorf("denied %q = %+v, %v", command, denied, err)
		}
	}
}
