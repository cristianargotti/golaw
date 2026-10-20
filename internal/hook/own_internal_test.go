package hook

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// ownMerge answers a merge in progress of m1 into c1 that staged shared.md
// and theirs.md as git merges them by itself, and the commits of the move
// from c0 to c1: c1, the command's own, changed shared.md in the second the
// pre event started; each question is recorded.
func ownMerge(second int64, asked *[]string) Dependencies {
	return Dependencies{
		AutomaticWith: func(_ context.Context, _ Repository, ours, theirs string, _, _ []string) (map[string][]string, error) {
			*asked = append(*asked, "merge "+ours+".."+theirs)
			return map[string][]string{"shared.md": {"b-shared", ""}, "theirs.md": {"b-theirs", "b-theirs"}}, nil
		},
		Committer: func(context.Context, Repository) (string, error) {
			*asked = append(*asked, "var")
			return "me <me@example.com>", nil
		},
		Commits: func(_ context.Context, _ Repository, since, now string, _ int64) ([]Commit, error) {
			*asked = append(*asked, "log "+since+".."+now)
			return []Commit{{Name: "c1", Committer: "me <me@example.com>", Time: second, Paths: []string{"shared.md"}}}, nil
		},
	}
}

// TestFromMergeKeepsWhatACommitOfTheCommandChanged judges a file a merge in
// progress staged as git merges it by itself when the command's own commit,
// made before it started the merge, changed it, and names the file only the
// merge changed as brought; git log and git var are asked once for the
// event, the move reading them from its memory, never when HEAD did not
// move, and when git cannot tell the commits of the command each file the
// merge staged is a gap.
func TestFromMergeKeepsWhatACommitOfTheCommandChanged(t *testing.T) {
	t.Parallel()
	started := time.Date(2026, time.October, 19, 12, 0, 0, 0, time.UTC)
	changes := Changes{
		Head: "c1", Merge: "m1", Staged: map[string]bool{"shared.md": true, "theirs.md": true},
		Index: map[string]string{"shared.md": "b-shared", "theirs.md": "b-theirs"},
	}
	staged := []string{"shared.md", "theirs.md"}
	before := Baseline{Files: map[string]string{}, Head: "c0", At: started}
	run := execution{repository: Repository{Head: "c1"}, own: &commitMemory{}}
	var asked []string
	deps := ownMerge(started.Unix(), &asked)
	got, gaps := run.fromMerge(t.Context(), deps, before, changes, slices.Clone(staged))
	if !slices.Equal(got, []string{"shared.md"}) || !slices.Equal(gaps, pathGaps([]string{"theirs.md"}, causeMerged)) ||
		!slices.Equal(asked, []string{"merge c1..m1", "var", "log c0..c1"}) {
		t.Fatalf("a commit of the command, then a merge: judged %v, gaps %+v after %q", got, gaps, asked)
	}
	if own := run.ownCommits(t.Context(), deps, before); !own.ok || !own.paths["shared.md"] || len(asked) != 3 {
		t.Fatalf("the memory answered %+v after %q", own, asked)
	}
	asked = nil
	still := before
	still.Head = "c1"
	got, gaps = execution{repository: Repository{Head: "c1"}}.fromMerge(t.Context(), deps, still, changes, slices.Clone(staged))
	if len(got) != 0 || !slices.Equal(gaps, pathGaps(staged, causeMerged)) || !slices.Equal(asked, []string{"merge c1..m1"}) {
		t.Fatalf("HEAD in place: judged %v, gaps %+v after %q", got, gaps, asked)
	}
	failing := deps
	failing.Commits = func(context.Context, Repository, string, string, int64) ([]Commit, error) {
		return nil, model.ErrUnreadable
	}
	got, gaps = execution{repository: Repository{Head: "c1"}}.fromMerge(t.Context(), failing, before, changes, slices.Clone(staged))
	if len(got) != 0 || !slices.Equal(gaps, pathGaps(staged, causeCommits)) {
		t.Fatalf("commits git cannot tell: judged %v, gaps %+v", got, gaps)
	}
}

// TestResolutionsJudgeTheSideOfHeadTheCommandCommitted judges a file a merge
// the command started left unmerged with the bytes HEAD holds when the
// command's own commit, made before the merge, changed it, leaves silent one
// whose bytes HEAD held unchanged, and makes each a gap when git cannot tell
// the commits of the command.
func TestResolutionsJudgeTheSideOfHeadTheCommandCommitted(t *testing.T) {
	t.Parallel()
	started := time.Date(2026, time.October, 19, 12, 0, 0, 0, time.UTC)
	var asked []string
	deps := ownMerge(started.Unix(), &asked)
	deps.Conflicts = func(context.Context, Repository, string, string, []string, []string) (map[string]Conflict, error) {
		return map[string]Conflict{
			"other.md":  {Merged: []byte("<<<<<<< c1\nother\n=======\nmate\n>>>>>>> m1\n"), Theirs: []byte("mate\n"), Ours: []byte("other\n")},
			"shared.md": {Merged: []byte("<<<<<<< c1\nmine\n=======\nmate\n>>>>>>> m1\n"), Theirs: []byte("mate\n"), Ours: []byte("mine\n")},
		}, nil
	}
	changes := Changes{
		Head: "c1", Merge: "m1", Unmerged: map[string]bool{"other.md": true, "shared.md": true},
		Snapshot: proof.NewTree(map[string][]byte{"other.md": []byte("other\n"), "shared.md": []byte("mine\n")}),
	}
	before := Baseline{Files: map[string]string{}, Head: "c0", At: started}
	unmerged := []string{"other.md", "shared.md"}
	got, gaps := execution{repository: Repository{Head: "c1"}, own: &commitMemory{}}.fromMerge(t.Context(), deps, before, changes, slices.Clone(unmerged))
	if !slices.Equal(got, []string{"shared.md"}) || len(gaps) != 0 || !slices.Equal(asked, []string{"var", "log c0..c1"}) {
		t.Fatalf("the side of HEAD: judged %v, gaps %+v after %q", got, gaps, asked)
	}
	deps.Commits = func(context.Context, Repository, string, string, int64) ([]Commit, error) {
		return nil, model.ErrUnreadable
	}
	got, gaps = execution{repository: Repository{Head: "c1"}}.fromMerge(t.Context(), deps, before, changes, slices.Clone(unmerged))
	if len(got) != 0 || !slices.Equal(gaps, pathGaps(unmerged, causeCommits)) {
		t.Fatalf("commits git cannot tell: judged %v, gaps %+v", got, gaps)
	}
}
