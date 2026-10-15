package shell_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestPushIsANoteUnlessDenied(t *testing.T) {
	t.Parallel()
	push := judge(t, shell.Push(), "git push origin main")
	if c := codes(push); len(c) != 1 || c[0] != "push" || push.Findings[0].Severity != model.SeverityNote {
		t.Errorf("push under the core policy = %+v", push.Findings)
	}
	for _, clean := range []string{"git fetch origin", "git pull", "echo git push"} {
		if c := codes(judge(t, shell.Push(), clean)); len(c) != 0 {
			t.Errorf("%q = %v", clean, c)
		}
	}
	strict := proof.Policy()
	strict.Shell = model.ShellPolicy{DenyPush: true}
	denied, err := shell.Push().Run(t.Context(), input("bash -c 'git push'", strict, proof.NewTree(nil)))
	if err != nil || len(denied.Findings) != 1 || denied.Findings[0].Severity != model.SeverityDeny || denied.Findings[0].Message != "git push is denied by the policy; the person pushes" {
		t.Errorf("denied push = %+v, %v", denied, err)
	}
}
