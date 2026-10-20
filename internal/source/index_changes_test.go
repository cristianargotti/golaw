package source_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

// blobAt answers the object id a revision expression names.
func blobAt(r *gittest.Repo, expression string) string {
	return r.Git("rev-parse", expression)
}

func TestIndexChangesAgainstHead(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Commit("feat: base", map[string]string{"a.txt": "one\n", "b.txt": "b\n", "c.txt": "one\ntwo\nthree\n", "e.sh": "echo\n"})
	r.StageOnly("a.txt", "two\n", "three\n")
	r.StageDeletion("b.txt")
	r.StageRename("c.txt", "dir/c.txt")
	r.Git("update-index", "--chmod=+x", "e.sh")
	r.Stage(map[string]string{"new.txt": "new\n"})
	x := indexOf(t, r.Root, source.IndexOptions{})
	want := []model.Change{
		{Op: model.ChangeModify, OldPath: "a.txt", NewPath: "a.txt", Mode: "100644", Before: blobAt(r, "HEAD:a.txt"), After: blobAt(r, ":a.txt")},
		{Op: model.ChangeDelete, OldPath: "b.txt", Mode: "100644", Before: blobAt(r, "HEAD:b.txt")},
		{Op: model.ChangeRename, OldPath: "c.txt", NewPath: "dir/c.txt", Mode: "100644", Before: blobAt(r, "HEAD:c.txt"), After: blobAt(r, ":dir/c.txt")},
		{Op: model.ChangeModify, OldPath: "e.sh", NewPath: "e.sh", Mode: "100755", Before: blobAt(r, "HEAD:e.sh"), After: blobAt(r, ":e.sh")},
		{Op: model.ChangeAdd, NewPath: "new.txt", Mode: "100644", After: blobAt(r, ":new.txt")},
	}
	if got := x.Changes(); !slices.Equal(got, want) {
		t.Errorf("changes =\n%+v\nwant\n%+v", got, want)
	}
	staged := []model.File{
		{Path: "a.txt", Class: model.ClassTracked, Size: 4},
		{Path: "dir/c.txt", Class: model.ClassTracked, Size: 14},
		{Path: "e.sh", Class: model.ClassTracked, Size: 5},
		{Path: "new.txt", Class: model.ClassTracked, Size: 4},
	}
	if got := x.Staged(); !slices.Equal(got, staged) {
		t.Errorf("staged = %+v, want %+v", got, staged)
	}
	if x.MergeHead() != "" || len(x.Unmerged()) != 0 {
		t.Errorf("no merge is in progress: %q, %v", x.MergeHead(), x.Unmerged())
	}
}

func TestIndexChangesOfAnUnbornBranchAndAgainstABase(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Stage(map[string]string{"b.txt": "b\n", "a.txt": "a\n"})
	unborn := indexOf(t, r.Root, source.IndexOptions{})
	want := []model.Change{
		{Op: model.ChangeAdd, NewPath: "a.txt", Mode: "100644", After: blobAt(r, ":a.txt")},
		{Op: model.ChangeAdd, NewPath: "b.txt", Mode: "100644", After: blobAt(r, ":b.txt")},
	}
	if got := unborn.Changes(); !slices.Equal(got, want) {
		t.Errorf("unborn changes = %+v, want %+v", got, want)
	}
	first := r.Commit("feat: first", nil)
	r.Commit("feat: second", map[string]string{"c.txt": "c\n"})
	r.Stage(map[string]string{"d.txt": "d\n"})
	since := indexOf(t, r.Root, source.IndexOptions{Base: first})
	if got := pathsOf(since.Staged()); !slices.Equal(got, []string{"c.txt", "d.txt"}) {
		t.Errorf("staged against the first commit = %v", got)
	}
	repo, err := source.Discover(t.Context(), r.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{"no-such-revision", "--cached"} {
		if _, err := source.NewIndex(t.Context(), repo, source.IndexOptions{Base: base}); !errors.Is(err, source.ErrGit) {
			t.Errorf("base %s: %v, want ErrGit", base, err)
		}
	}
}

func TestIndexChangesLeaveOutIntentsToAdd(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Commit("feat: base", map[string]string{"a/.keep": "", "x.txt": "x\n"})
	r.StageDeletion("a/.keep")
	r.Write("b/.keep", "")
	r.Write("intent.txt", "intent\n")
	r.Git("add", "-N", "b/.keep", "intent.txt")
	x := indexOf(t, r.Root, source.IndexOptions{})
	want := []model.Change{{Op: model.ChangeDelete, OldPath: "a/.keep", Mode: "100644", Before: blobAt(r, "HEAD:a/.keep")}}
	if got := x.Changes(); !slices.Equal(got, want) {
		t.Errorf("changes = %+v, want %+v", got, want)
	}
	if got := x.Staged(); len(got) != 0 {
		t.Errorf("staged = %+v, want nothing", got)
	}
}

func TestIndexNamesTheMergeInProgress(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Commit("feat: base", map[string]string{"shared.md": "base\n"})
	tip := r.Branch("side", map[string]string{"shared.md": "theirs\n", "theirs.md": "theirs\n"})
	r.Commit("feat: ours", map[string]string{"shared.md": "ours\n"})
	r.MergeInProgress("side")
	x := indexOf(t, r.Root, source.IndexOptions{})
	if x.MergeHead() != tip {
		t.Errorf("merge head = %q, want %s", x.MergeHead(), tip)
	}
	if got := x.Unmerged(); !slices.Equal(got, []string{"shared.md"}) {
		t.Errorf("unmerged = %v", got)
	}
	want := []model.Change{{Op: model.ChangeAdd, NewPath: "theirs.md", Mode: "100644", After: blobAt(r, ":theirs.md")}}
	if got := x.Changes(); !slices.Equal(got, want) {
		t.Errorf("changes = %+v, want %+v", got, want)
	}
}
