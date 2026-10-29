package commits_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/commits"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// commitInput is the commit mode input of one commit over a tree, the
// changes empty: the message checks read the commit alone.
func commitInput(c model.Commit, snap model.Snapshot) model.Input {
	return model.Input{Mode: model.ModeCommit, Policy: proof.Policy(), Snapshot: snap, Commit: &c}
}

// judge runs one check over a commit whose message git cleans with the
// strip mode, as an edited message is cleaned, and requires it measured.
func judge(t *testing.T, c model.Check, message string) model.Result {
	t.Helper()
	return judgeCommit(t, c, model.Commit{Message: message})
}

// judgeCommit runs one check over a commit and requires it measured.
func judgeCommit(t *testing.T, c model.Check, commit model.Commit) model.Result {
	t.Helper()
	res, err := c.Run(t.Context(), commitInput(commit, proof.NewTree(nil)))
	if err != nil {
		t.Fatal(err)
	}
	if res.Status.Kind != model.StatusMeasured {
		t.Fatalf("%q: status %+v", commit.Message, res.Status)
	}
	return res
}

// codes lists the finding codes of a result in order.
func codes(res model.Result) []string {
	out := make([]string, 0, len(res.Findings))
	for k := range res.Findings {
		f := &res.Findings[k]
		out = append(out, f.Code)
	}
	return out
}

// denies runs a table of messages and checks which ones yield the code,
// once.
func denies(t *testing.T, c model.Check, code string, cases map[string]bool) {
	t.Helper()
	for message, want := range cases {
		got := codes(judge(t, c, message))
		if (len(got) == 1 && got[0] == code) != want || len(got) > 1 {
			t.Errorf("%q: codes %v, want finding %v", message, got, want)
		}
	}
}

func TestChecksAreCoreCommitChecks(t *testing.T) {
	t.Parallel()
	for _, c := range commits.Checks() {
		s := c.Spec()
		if err := s.Validate(); err != nil {
			t.Errorf("%s: %v", s.ID, err)
		}
		if s.Proof != s.ID || s.Scope != model.ScopeCommit || s.Owner != model.OwnerCore || s.Rule != "rules/4.12@1" || len(s.Needs) != 0 {
			t.Errorf("%s: spec = %+v", s.ID, s)
		}
		res, err := c.Run(t.Context(), model.Input{Mode: model.ModeCommit, Policy: proof.Policy(), Snapshot: proof.NewTree(nil)})
		if err != nil || res.Status.Kind != model.StatusNotApplicable {
			t.Errorf("%s without a commit: %+v, %v", s.ID, res, err)
		}
	}
}
