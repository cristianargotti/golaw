package hook

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

// TestMovesBroughtWhatGitStatusReportsCleanAfterIt names, of the changed
// paths, those the move of HEAD changed that git status reports clean after
// the command and that are still on disk: a path git status names, one the
// move left alone and one the move deleted are not among them.
func TestMovesBroughtWhatGitStatusReportsCleanAfterIt(t *testing.T) {
	t.Parallel()
	moved := moves{paths: map[string]string{"theirs.md": "", "named.md": "b", "gone.md": "b"}, known: true}
	after := map[string]string{"theirs.md": "x", "named.md": "y", "gone.md": absentIdentity, "dirty.md": "z"}
	got := moved.brought(Changes{Paths: []string{"named.md"}}, []string{"dirty.md", "gone.md", "named.md", "theirs.md"}, after)
	if !slices.Equal(got, []string{"theirs.md"}) {
		t.Fatalf("brought = %v", got)
	}
}

// createdDeps answer the committer and the commits of a move: one of the
// command in the second its pre event started, a teammate's of an hour
// before and one of an hour after, and one of the same identity a second
// before the pre event, recording each question.
func createdDeps(second int64, asked *[]string) Dependencies {
	return Dependencies{
		Committer: func(context.Context, Repository) (string, error) {
			*asked = append(*asked, "var")
			return "me <me@example.com>", nil
		},
		Commits: func(_ context.Context, _ Repository, since, now string, from int64) ([]Commit, error) {
			*asked = append(*asked, since+".."+now+"@"+strconv.FormatInt(from, 10))
			return []Commit{
				{Committer: "me <me@example.com>", Time: second, Paths: []string{"mine.md"}},
				{Committer: "mate <mate@example.com>", Time: second - 3600, Paths: []string{"theirs.md"}},
				{Committer: "mate <mate@example.com>", Time: second + 3600, Paths: []string{"future.md"}},
				{Committer: "me <me@example.com>", Time: second - 1, Paths: []string{"older.md"}},
			}, nil
		},
	}
}

// TestFromCommitsJudgesOnlyWhatTheCommandCommitted keeps, of the paths a
// move brought, those a commit of the command changed, one committed by the
// identity git var answers in the whole second the pre event started or
// later, and names each other one as arrived with the move: a teammate's of
// before or after and one of the same identity a second before; it asks git
// nothing when the move brought nothing, and when git cannot tell which
// commits the command created each path it brought is a gap.
func TestFromCommitsJudgesOnlyWhatTheCommandCommitted(t *testing.T) {
	t.Parallel()
	started := time.Date(2026, time.October, 17, 12, 0, 0, 900_000_000, time.UTC)
	before := Baseline{Head: "c1", At: started}
	var asked []string
	deps := createdDeps(started.Unix(), &asked)
	run := execution{repository: Repository{Head: "c2"}}
	changed := []string{"dirty.md", "future.md", "mine.md", "older.md", "theirs.md"}
	brought := changed[1:]
	got, gaps := run.fromCommits(t.Context(), deps, before, slices.Clone(changed), brought)
	arrived := pathGaps([]string{"future.md", "older.md", "theirs.md"}, causeArrived)
	if !slices.Equal(got, []string{"dirty.md", "mine.md"}) || !slices.Equal(gaps, arrived) ||
		!slices.Equal(asked, []string{"var", "c1..c2@" + strconv.FormatInt(started.Unix(), 10)}) {
		t.Fatalf("judged %v, gaps %+v after %v", got, gaps, asked)
	}
	if got, gaps := run.fromCommits(t.Context(), deps, before, []string{"dirty.md"}, nil); !slices.Equal(got, []string{"dirty.md"}) || len(gaps) != 0 || len(asked) != 2 {
		t.Fatalf("nothing brought: judged %v, gaps %+v after %v", got, gaps, asked)
	}
	broken := map[string]Dependencies{
		"git var fails": {Commits: deps.Commits, Committer: func(context.Context, Repository) (string, error) { return "", model.ErrUnreadable }},
		"git log fails": {Committer: deps.Committer, Commits: func(context.Context, Repository, string, string, int64) ([]Commit, error) {
			return nil, model.ErrUnreadable
		}},
		"no binding": {},
	}
	for name, failing := range broken {
		got, gaps := run.fromCommits(t.Context(), failing, before, slices.Clone(changed), brought)
		if !slices.Equal(got, []string{"dirty.md"}) || !slices.Equal(gaps, pathGaps(brought, causeCommits)) {
			t.Fatalf("%s: judged %v, gaps %+v", name, got, gaps)
		}
	}
}

// TestTheNoticeNamesWhatAMoveBrought gives the files a move brought one
// sentence of their own, after what was not measured, naming up to twenty
// of them and counting the rest, in English for the agent and in the
// language of the policy for the person.
func TestTheNoticeNamesWhatAMoveBrought(t *testing.T) {
	t.Parallel()
	gaps := append([]Gap{OverReadBound("big.sql")}, pathGaps([]string{"a.md", "b.md"}, causeArrived)...)
	text := "golaw did not measure big.sql: it is over the 8 MiB the hook reads; the pipeline judges it. " +
		"golaw did not judge the files the move of HEAD brought from commits this command did not create, which ask for no fix: a.md, b.md; the pipeline judges them."
	human := "golaw n\u00e3o mediu big.sql, que o pipeline julga. " +
		"golaw n\u00e3o julgou os arquivos que a mudan\u00e7a de HEAD trouxe de commits que este comando n\u00e3o criou, que n\u00e3o pedem corre\u00e7\u00e3o: a.md, b.md; o pipeline os julga."
	if got := noticed(gaps, model.DefaultLanguage); got.Outcome != Allow || got.Notice != text || got.Human != human {
		t.Fatalf("notice %q, human %q", got.Notice, got.Human)
	}
	many := make([]string, 25)
	for i := range many {
		many[i] = "f" + strconv.Itoa(i+10) + ".md"
	}
	got := noticed(pathGaps(many, causeArrived), "en")
	listed := strings.Join(many[:20], ", ")
	if !strings.Contains(got.Notice, ": "+listed+" and 5 more; the pipeline judges them.") ||
		got.Human != "golaw did not judge the files the move of HEAD brought from commits this command did not create, which ask for no fix: "+listed+" and 5 more; the pipeline judges them." {
		t.Fatalf("many arrived: notice %q, human %q", got.Notice, got.Human)
	}
	if pt := noticed(pathGaps(many, causeArrived), model.DefaultLanguage); !strings.HasSuffix(pt.Human, listed+" e mais 5; o pipeline os julga.") {
		t.Fatalf("many arrived in Portuguese: %q", pt.Human)
	}
}

// mergeDeps answer the committer and the commits of a move whose command
// made a merge m1 in the second its pre event started, with the paths m1
// holds apart from every parent, and a commit c3 of its own beside a
// teammate's merge and commit of an hour before; Remerged answers the paths
// m1 holds apart from the merge git makes of its parents by itself. Each
// question is recorded.
func mergeDeps(second int64, asked *[]string) Dependencies {
	return Dependencies{
		Committer: func(context.Context, Repository) (string, error) { return "me <me@example.com>", nil },
		Commits: func(context.Context, Repository, string, string, int64) ([]Commit, error) {
			*asked = append(*asked, "log")
			return []Commit{
				{Name: "m1", Committer: "me <me@example.com>", Time: second, Merge: true, Paths: []string{"auto.md", "mine.md", "other.md", "shared.md"}},
				{Name: "m0", Committer: "mate <mate@example.com>", Time: second - 3600, Merge: true, Paths: []string{"theirs.md"}},
				{Name: "c3", Committer: "me <me@example.com>", Time: second, Paths: []string{"own.md"}},
				{Name: "t1", Committer: "mate <mate@example.com>", Time: second - 3600, Paths: []string{"theirs.md"}},
			}, nil
		},
		Remerged: func(_ context.Context, _ Repository, merges []string) (map[string][]string, error) {
			*asked = append(*asked, "remerge "+strings.Join(merges, ","))
			return map[string][]string{"m1": {"extra.md", "mine.md", "shared.md"}}, nil
		},
	}
}

// TestFromCommitsJudgesWhatAMergeOfTheCommandHolds judges, of the paths a
// move brought, those a merge the command made holds apart both from every
// parent and from the merge git makes of its parents by itself, the
// command's resolution and its own files, beside those of its other
// commits, and names as arrived a path that merge took from one side whole
// or that git merged by itself; git is asked the remerge once, only when
// such a merge holds a path the move brought, and when it cannot answer,
// each path that merge holds apart from every parent is a gap.
func TestFromCommitsJudgesWhatAMergeOfTheCommandHolds(t *testing.T) {
	t.Parallel()
	started := time.Date(2026, time.October, 17, 12, 0, 0, 0, time.UTC)
	before := Baseline{Head: "c1", At: started}
	run := execution{repository: Repository{Head: "m1"}}
	changed := []string{"auto.md", "dirty.md", "mine.md", "own.md", "shared.md", "taken.md", "theirs.md"}
	brought := []string{"auto.md", "mine.md", "own.md", "shared.md", "taken.md", "theirs.md"}
	var asked []string
	deps := mergeDeps(started.Unix(), &asked)
	got, gaps := run.fromCommits(t.Context(), deps, before, slices.Clone(changed), brought)
	if !slices.Equal(got, []string{"dirty.md", "mine.md", "own.md", "shared.md"}) ||
		!slices.Equal(gaps, pathGaps([]string{"auto.md", "taken.md", "theirs.md"}, causeArrived)) || !slices.Equal(asked, []string{"log", "remerge m1"}) {
		t.Fatalf("judged %v, gaps %+v after %v", got, gaps, asked)
	}
	asked = nil
	if got, gaps := run.fromCommits(t.Context(), deps, before, []string{"taken.md"}, []string{"taken.md"}); len(got) != 0 ||
		!slices.Equal(gaps, pathGaps([]string{"taken.md"}, causeArrived)) || !slices.Equal(asked, []string{"log"}) {
		t.Fatalf("a merge that holds no path the move brought: judged %v, gaps %+v after %v", got, gaps, asked)
	}
	failing := deps
	failing.Remerged = func(context.Context, Repository, []string) (map[string][]string, error) {
		return nil, model.ErrUnreadable
	}
	unbound := deps
	unbound.Remerged = nil
	for name, broken := range map[string]Dependencies{"git cannot remerge": failing, "no binding": unbound} {
		got, gaps := run.fromCommits(t.Context(), broken, before, slices.Clone(changed), brought)
		want := append(pathGaps([]string{"auto.md", "mine.md", "shared.md"}, causeRemergeUnknown), pathGaps([]string{"taken.md", "theirs.md"}, causeArrived)...)
		if !slices.Equal(got, []string{"dirty.md", "own.md"}) || !slices.Equal(gaps, want) {
			t.Fatalf("%s: judged %v, gaps %+v", name, got, gaps)
		}
	}
}
