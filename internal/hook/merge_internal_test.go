package hook

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

// mergeChanges are the changes git status reports while a merge of m1 is
// in progress: four paths staged with their bytes on disk and the blob the
// index holds at each, two it left unmerged, one of them already named
// before the command.
func mergeChanges() Changes {
	return Changes{
		Head: "c1", Merge: "m1",
		Staged: map[string]bool{"auto.md": true, "mine.md": true, "taken.md": true, "theirs.md": true},
		Index: map[string]string{
			"auto.md": "b-auto", "mine.md": "b-mine", "taken.md": "b-taken", "theirs.md": "b-theirs",
		},
		Unmerged: map[string]bool{"dirty.md": true, "shared.md": true},
	}
}

// automaticMerge answers the blobs the merge git makes of c1 and m1 by
// itself and m1 hold at each staged path, recording each question: auto.md
// as git merged it, theirs.md as m1 brought it, taken.md whose conflict
// the command resolved with the side of m1 whole, and mine.md, which only
// the local side holds, whose staged blob is neither.
func automaticMerge(asked *[]string) Dependencies {
	return Dependencies{Automatic: func(_ context.Context, _ Repository, ours, theirs string, paths []string) (map[string][]string, error) {
		*asked = append(*asked, ours+".."+theirs+" "+strings.Join(paths, ","))
		return map[string][]string{
			"auto.md": {"b-auto", "b-side"}, "mine.md": {"b-local", ""},
			"taken.md": {"b-conflict", "b-taken"}, "theirs.md": {"b-theirs", "b-theirs"},
		}, nil
	}}
}

// TestFromMergeLeavesWhatGitWrote leaves out of what a command changed what
// git wrote for it: a path git left unmerged that git status did not name
// before the command is a gap until its resolution is written, and, when
// the command started the merge, a path the merge staged whose staged blob
// is the one the merge git makes by itself holds there, or the one the
// merged commit holds, is named as brought by the merge; a path whose
// staged blob is neither, one the command wrote after the merge and one
// already named stay judged.
func TestFromMergeLeavesWhatGitWrote(t *testing.T) {
	t.Parallel()
	before := Baseline{Files: map[string]string{"dirty.md": "x"}, Head: "c1"}
	changed := []string{"auto.md", "dirty.md", "edited.md", "mine.md", "shared.md", "taken.md", "theirs.md"}
	run := execution{repository: Repository{Head: "c1"}}
	var asked []string
	got, gaps := run.fromMerge(t.Context(), automaticMerge(&asked), before, mergeChanges(), slices.Clone(changed))
	unmerged := pathGaps([]string{"shared.md"}, causeUnmerged)
	want := append(slices.Clone(unmerged), pathGaps([]string{"auto.md", "taken.md", "theirs.md"}, causeMerged)...)
	if !slices.Equal(got, []string{"dirty.md", "edited.md", "mine.md"}) || !slices.Equal(gaps, want) ||
		!slices.Equal(asked, []string{"c1..m1 auto.md,mine.md,taken.md,theirs.md"}) {
		t.Fatalf("a merge the command started: judged %v, gaps %+v after %v", got, gaps, asked)
	}
	before.Merge = "m1"
	got, gaps = run.fromMerge(t.Context(), automaticMerge(&asked), before, mergeChanges(), slices.Clone(changed))
	if !slices.Equal(got, []string{"auto.md", "dirty.md", "edited.md", "mine.md", "taken.md", "theirs.md"}) || !slices.Equal(gaps, unmerged) || len(asked) != 1 {
		t.Fatalf("a merge in progress before the command: judged %v, gaps %+v after %v", got, gaps, asked)
	}
	before.Merge = ""
	failing := Dependencies{Automatic: func(context.Context, Repository, string, string, []string) (map[string][]string, error) {
		return nil, model.ErrUnreadable
	}}
	octopus := mergeChanges()
	octopus.Merge = "m1\nm2"
	for name, tc := range map[string]struct {
		deps    Dependencies
		changes Changes
	}{"git cannot merge": {failing, mergeChanges()}, "no binding": {Dependencies{}, mergeChanges()}, "an octopus": {automaticMerge(&asked), octopus}} {
		got, gaps = run.fromMerge(t.Context(), tc.deps, before, tc.changes, slices.Clone(changed))
		want = append(slices.Clone(unmerged), pathGaps([]string{"auto.md", "mine.md", "taken.md", "theirs.md"}, causeMergeUnknown)...)
		if !slices.Equal(got, []string{"dirty.md", "edited.md"}) || !slices.Equal(gaps, want) || len(asked) != 1 {
			t.Fatalf("%s: judged %v, gaps %+v after %v", name, got, gaps, asked)
		}
	}
}

// TestTheNoticeNamesWhatAMergeBrought gives the files a merge in progress
// brought a sentence of their own after the files a move brought, and names
// a file git left unmerged as a file the event of its resolution judges, in
// English for the agent and in the language of the policy for the person.
func TestTheNoticeNamesWhatAMergeBrought(t *testing.T) {
	t.Parallel()
	gaps := append(pathGaps([]string{"shared.md"}, causeUnmerged), pathGaps([]string{"theirs.md"}, causeMerged)...)
	gaps = append(gaps, pathGaps([]string{"moved.md"}, causeArrived)...)
	text := "golaw did not measure shared.md: git left it unmerged, and golaw judges the resolution the agent writes; the pipeline judges it. " +
		"golaw did not judge the files the move of HEAD brought from commits this command did not create, which ask for no fix: moved.md; the pipeline judges them. " +
		"golaw did not judge the files the merge in progress brought from commits this command did not create, which ask for no fix: theirs.md; the pipeline judges them."
	human := "golaw n\u00e3o mediu shared.md, que o pipeline julga. " +
		"golaw n\u00e3o julgou os arquivos que a mudan\u00e7a de HEAD trouxe de commits que este comando n\u00e3o criou, que n\u00e3o pedem corre\u00e7\u00e3o: moved.md; o pipeline os julga. " +
		"golaw n\u00e3o julgou os arquivos que o merge em andamento trouxe de commits que este comando n\u00e3o criou, que n\u00e3o pedem corre\u00e7\u00e3o: theirs.md; o pipeline os julga."
	if got := noticed(gaps, model.DefaultLanguage); got.Outcome != Allow || got.Notice != text || got.Human != human {
		t.Fatalf("notice %q, human %q", got.Notice, got.Human)
	}
}

// TestTheBaselineKeepsTheMergeInProgress saves the commits MERGE_HEAD
// named at the pre event with its baseline and loads them back.
func TestTheBaselineKeepsTheMergeInProgress(t *testing.T) {
	t.Parallel()
	store := Store{Dir: t.TempDir(), Now: time.Date(2026, time.October, 17, 12, 0, 0, 0, time.UTC)}
	if err := store.Save(t.Context(), "key", "input", Baseline{Files: map[string]string{}, Head: "c1", Merge: "m1"}); err != nil {
		t.Fatal(err)
	}
	if loaded, err := store.Load(t.Context(), "key"); err != nil || loaded.Merge != "m1" || loaded.Head != "c1" {
		t.Fatalf("loaded %+v, %v", loaded, err)
	}
}
