package hook

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

// amendDeps answer the commits of a move from h1, made on p0: n1, the
// command's amend of h1 on p0, which changed keep.md against p0 and left
// notes.md as p0 holds it; Parents names p0 as the parent of h1 and Moved
// what n1 changed against h1. Each question is recorded.
func amendDeps(second int64, asked *[]string) Dependencies {
	return Dependencies{
		Committer: func(context.Context, Repository) (string, error) { return "me <me@example.com>", nil },
		Commits: func(context.Context, Repository, string, string, int64) ([]Commit, error) {
			return []Commit{{Name: "n1", Committer: "me <me@example.com>", Time: second, Paths: []string{"keep.md"}, Parents: []string{"p0"}}}, nil
		},
		Parents: func(_ context.Context, _ Repository, commit string) ([]string, error) {
			*asked = append(*asked, "parents "+commit)
			return []string{"p0"}, nil
		},
		Moved: func(_ context.Context, _ Repository, from, to string) (map[string]string, error) {
			*asked = append(*asked, "moved "+from+".."+to)
			return map[string]string{"notes.md": "b-fixed", "keep.md": "b-keep"}, nil
		},
	}
}

// TestUnrewrittenJudgesWhatAnAmendChanged judges a file the command's amend
// restored to the bytes its parent holds, as every file the move from the
// amended HEAD changed when the amend is HEAD, asking git the parents of
// that HEAD once, and only the files the amend changed, asked of git
// diff-tree, when a commit follows it; a teammate's file of the same move
// stays arrived.
func TestUnrewrittenJudgesWhatAnAmendChanged(t *testing.T) {
	t.Parallel()
	started := time.Date(2026, time.October, 19, 12, 0, 0, 0, time.UTC)
	before := Baseline{Head: "h1", At: started, Amend: true}
	brought := []string{"keep.md", "notes.md"}
	var asked []string
	got, gaps := execution{repository: Repository{Head: "n1"}}.fromCommits(t.Context(), amendDeps(started.Unix(), &asked), before, slices.Clone(brought), brought)
	if !slices.Equal(got, brought) || len(gaps) != 0 || !slices.Equal(asked, []string{"parents h1"}) {
		t.Fatalf("an amend at HEAD: judged %v, gaps %+v after %q", got, gaps, asked)
	}
	asked = nil
	moved := []string{"keep.md", "notes.md", "theirs.md"}
	got, gaps = execution{repository: Repository{Head: "t1"}}.fromCommits(t.Context(), amendDeps(started.Unix(), &asked), before, slices.Clone(moved), moved)
	if !slices.Equal(got, brought) || !slices.Equal(gaps, pathGaps([]string{"theirs.md"}, causeArrived)) ||
		!slices.Equal(asked, []string{"parents h1", "moved h1..n1"}) {
		t.Fatalf("an amend before a teammate's commit: judged %v, gaps %+v after %q", got, gaps, asked)
	}
}

// TestUnrewrittenLeavesWhatNoAmendChanged asks git nothing when a commit of
// the command builds on the HEAD of the pre event or no commit is the
// command's, leaves a file arrived when no commit of the command is made on
// the parents of that HEAD, and makes each file a gap when git cannot name
// the parents or the files of the amend.
func TestUnrewrittenLeavesWhatNoAmendChanged(t *testing.T) {
	t.Parallel()
	started := time.Date(2026, time.October, 19, 12, 0, 0, 0, time.UTC)
	before := Baseline{Head: "h1", At: started, Amend: true}
	var asked []string
	run := execution{repository: Repository{Head: "n2"}}
	for name, tc := range map[string]struct {
		mine []Commit
		want []string
	}{
		"a commit of the command on the HEAD of the pre event": {[]Commit{{Name: "c2", Parents: []string{"h1"}}, {Name: "n1", Parents: []string{"p0"}}}, []string{"notes.md"}},
		"no commit of the command":                             {nil, []string{"notes.md"}},
		"a commit on other parents":                            {[]Commit{{Name: "c2", Parents: []string{"x9"}}}, []string{"notes.md"}},
	} {
		asked = nil
		arrived, uncertain := run.unrewritten(t.Context(), amendDeps(started.Unix(), &asked), before, tc.mine, []string{"notes.md"})
		if !slices.Equal(arrived, tc.want) || len(uncertain) != 0 {
			t.Fatalf("%s: arrived %v, uncertain %v after %q", name, arrived, uncertain, asked)
		}
	}
	failing := amendDeps(started.Unix(), &asked)
	failing.Parents = func(context.Context, Repository, string) ([]string, error) { return nil, model.ErrUnreadable }
	noMove := amendDeps(started.Unix(), &asked)
	noMove.Moved = func(context.Context, Repository, string, string) (map[string]string, error) {
		return nil, model.ErrUnreadable
	}
	unbound := amendDeps(started.Unix(), &asked)
	unbound.Moved = nil
	mine := []Commit{{Name: "n1", Parents: []string{"p0"}}}
	asked = nil
	plain := Baseline{Head: "h1", At: started}
	if arrived, uncertain := run.unrewritten(t.Context(), amendDeps(started.Unix(), &asked), plain, mine, []string{"notes.md"}); !slices.Equal(arrived, []string{"notes.md"}) || len(uncertain) != 0 || len(asked) != 0 {
		t.Fatalf("a commit on the parents by a command that does not amend: arrived %v, uncertain %v after %q", arrived, uncertain, asked)
	}
	for name, deps := range map[string]Dependencies{"git cannot name the parents": failing, "git cannot name the files of the amend": noMove, "no binding of the move": unbound} {
		arrived, uncertain := run.unrewritten(t.Context(), deps, before, mine, []string{"notes.md"})
		if len(arrived) != 0 || !slices.Equal(uncertain, []string{"notes.md"}) {
			t.Fatalf("%s: arrived %v, uncertain %v", name, arrived, uncertain)
		}
	}
}

// TestAmendsReadsTheAmendOfAGitCommit reads --amend only in a git commit
// the command text runs plainly, outside every quoted span: a reset
// followed by a commit, whose message may name --amend in single or double
// quotes, an amend a script runs and an echo of the words are not amends,
// while an amend after a quoted message, a separator or a backslash inside
// quotes, a quoted global option or a quoted --amend itself is one.
func TestAmendsReadsTheAmendOfAGitCommit(t *testing.T) {
	t.Parallel()
	for command, want := range map[string]bool{
		"git commit -q --amend --no-edit":                          true,
		"git checkout -q HEAD~1 -- notes.md && git commit --amend": true,
		"git -C repo commit -a --amend":                            true,
		"git reset -q --hard HEAD~1 && git commit -m again":        false,
		"sh amend.sh":             false,
		"echo git commit --amend": false,
		"git commit -q -m again":  false,
		"git reset -q --hard HEAD~1 && git commit -q --allow-empty -m 'docs: explain --amend behavior'": false,
		`git reset -q --hard HEAD~1 && git commit -q --allow-empty -m "docs: explain --amend behavior"`: false,
		`git commit -m "fix; git commit --amend"`:                                                       false,
		`git commit -m "say \"it\" --amend"`:                                                            false,
		"git commit -m 'it is; done' --amend":                                                           true,
		`git commit -m "say \"it\"" --amend`:                                                            true,
		"git -c 'user.name=A B' commit --amend":                                                         true,
		"git commit '--amend'":                                                                          true,
		"git commit \\\n  --amend":                                                                      true,
	} {
		if got := amends(command); got != want {
			t.Errorf("%q: amends %v, want %v", command, got, want)
		}
	}
}
