package policy_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// staged is what an index holds at golaw.yml when stage 0 holds the text.
func staged(text string) policy.StagedPolicy {
	return policy.StagedPolicy{Data: []byte(text), Found: true}
}

// inForce loads the policy in force of the repository at root.
func inForce(t *testing.T, root string, d policy.Dirs) *policy.Loaded {
	t.Helper()
	l, err := policy.Load(t.Context(), input(root, d))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestEvaluateProposalJudgesTheStagedBytesApart(t *testing.T) {
	t.Parallel()
	root, d := enrolled(t, fullPolicy)
	l := inForce(t, root, d)
	same, err := policy.EvaluateProposal(l, staged(fullPolicy), schemas())
	if err != nil || same.Found || !same.Staged {
		t.Errorf("the policy in force staged again = %+v, %v", same, err)
	}
	tighter := strings.Replace(fullPolicy, "file_lines: 250", "file_lines: 200", 1)
	next, err := policy.EvaluateProposal(l, staged(tighter), schemas())
	if err != nil || !next.Found || !next.Staged || *next.File.Checks.Sizes.FileLines != 200 {
		t.Errorf("a staged tightening = %+v, %v", next, err)
	}
	rung := strings.Replace(fullPolicy, "pre_push: false", "pre_push: true", 1)
	if free, err := policy.EvaluateProposal(l, staged(rung), schemas()); err != nil || free.Found {
		t.Errorf("a rung switched in the ladder is no new policy: %+v, %v", free, err)
	}
}

func TestEvaluateProposalRefusesWhatWouldWeakenThePolicyInForce(t *testing.T) {
	t.Parallel()
	root, d := enrolled(t, fullPolicy)
	l := inForce(t, root, d)
	cases := map[string]struct {
		staged policy.StagedPolicy
		want   error
		says   []string
	}{
		"a weakened floor": {
			staged(strings.Replace(fullPolicy, "file_lines: 250", "file_lines: 280", 1)), model.ErrPolicy,
			[]string{"checks.sizes.file_lines: 250 to 280", "restore what the policy in force holds", "git restore --staged golaw.yml"},
		},
		"a broken schema":     {staged("golaw: [\n"), model.ErrPolicy, []string{"git restore --staged golaw.yml"}},
		"an unresolved merge": {policy.StagedPolicy{Unmerged: true}, model.ErrPolicy, []string{"golaw.yml is unresolved in the index", "finish the merge"}},
		"a staged deletion": {
			policy.StagedPolicy{},
			model.ErrEnrollment,
			[]string{"the deletion of golaw.yml is staged", "git restore --staged --worktree golaw.yml"},
		},
	}
	for name, tc := range cases {
		_, err := policy.EvaluateProposal(l, tc.staged, schemas())
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: %v, want %v", name, err, tc.want)
			continue
		}
		for _, phrase := range tc.says {
			if !strings.Contains(err.Error(), phrase) {
				t.Errorf("%s: %q lacks %q", name, err, phrase)
			}
		}
	}
}

func TestEvaluateProposalLeavesADeletionToTheEnrollment(t *testing.T) {
	t.Parallel()
	d := dirs(t)
	unenrolled := initRepo(t)
	write(t, unenrolled, "golaw.yml", minimalPolicy)
	commitAll(t, unenrolled, "policy")
	if p, err := policy.EvaluateProposal(inForce(t, unenrolled, d), policy.StagedPolicy{}, schemas()); err != nil || p.Found || p.Staged {
		t.Errorf("an unenrolled repository may stage the deletion: %+v, %v", p, err)
	}
	unborn := initRepo(t)
	write(t, unborn, "golaw.yml", minimalPolicy)
	writeRecord(t, d.State, policy.Record{
		Root: unborn, PolicyDigest: policy.Digest(decode(t, minimalPolicy)), KitVersion: "0.1.0",
		Bootstrap: minimalPolicy, WrittenBy: policy.WrittenByInit, WrittenAt: now(),
	})
	if p, err := policy.EvaluateProposal(inForce(t, unborn, d), policy.StagedPolicy{}, schemas()); err != nil || p.Found {
		t.Errorf("the bootstrap record governs before the first commit: %+v, %v", p, err)
	}
	core := initRepo(t)
	write(t, core, "a.txt", "a\n")
	commitAll(t, core, "one")
	if p, err := policy.EvaluateProposal(inForce(t, core, d), staged(fullPolicy), schemas()); err != nil || !p.Found {
		t.Errorf("a first policy only needs to be valid: %+v, %v", p, err)
	}
}
