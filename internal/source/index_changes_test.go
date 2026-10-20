package source_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
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
	for _, base := range []string{"no-such-revision", "--cached", "HEAD^{tree}"} {
		if _, err := source.NewIndex(t.Context(), repo, source.IndexOptions{Base: base}); !errors.Is(err, source.ErrGit) {
			t.Errorf("base %s: %v, want ErrGit", base, err)
		}
	}
}

func TestIndexChangesLeaveOutIntentsToAdd(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Commit("feat: base", map[string]string{"a/.keep": "", "c/.keep": "", "x.txt": "x\n"})
	r.StageDeletion("a/.keep")
	r.Git("rm", "-q", "--cached", "c/.keep")
	r.Write("b/.keep", "")
	r.Write("intent.txt", "intent\n")
	r.Git("add", "-N", "b/.keep", "c/.keep", "intent.txt")
	x := indexOf(t, r.Root, source.IndexOptions{})
	want := []model.Change{
		{Op: model.ChangeDelete, OldPath: "a/.keep", Mode: "100644", Before: blobAt(r, "HEAD:a/.keep")},
		{Op: model.ChangeDelete, OldPath: "c/.keep", Mode: "100644", Before: blobAt(r, "HEAD:c/.keep")},
	}
	if got := x.Changes(); !slices.Equal(got, want) {
		t.Errorf("changes = %+v, want %+v", got, want)
	}
	if got := x.Staged(); len(got) != 0 {
		t.Errorf("staged = %+v, want nothing", got)
	}
	if got := pathsOf(x.Files()); !slices.Equal(got, []string{"x.txt"}) {
		t.Errorf("files = %v, want x.txt alone, as the next commit records", got)
	}
}

// TestIndexChangesLeaveOutAnIntentWhoseFileIsGone holds the universe and
// the changes to what the next commit records when the file of an intent
// to add left the worktree, beside an empty file that is truly staged: on
// a branch with a history, where an intent at a path HEAD holds is its
// deletion, on an unborn branch and in a repository of the other object
// format.
func TestIndexChangesLeaveOutAnIntentWhoseFileIsGone(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Commit("feat: base", map[string]string{"a.txt": "a\n", "held.txt": "held\n"})
	r.Git("rm", "-q", "--cached", "held.txt")
	r.Write("gone.png", "png\n")
	r.Git("add", "-N", "gone.png", "held.txt")
	r.Remove("gone.png")
	r.Remove("held.txt")
	r.Stage(map[string]string{"empty.txt": ""})
	x := indexOf(t, r.Root, source.IndexOptions{})
	want := []model.Change{
		{Op: model.ChangeAdd, NewPath: "empty.txt", Mode: "100644", After: blobAt(r, ":empty.txt")},
		{Op: model.ChangeDelete, OldPath: "held.txt", Mode: "100644", Before: blobAt(r, "HEAD:held.txt")},
	}
	if got := x.Changes(); !slices.Equal(got, want) {
		t.Errorf("changes = %+v, want %+v", got, want)
	}
	if got := pathsOf(x.Staged()); !slices.Equal(got, []string{"empty.txt"}) {
		t.Errorf("staged = %v, want empty.txt alone", got)
	}
	r.Git("commit", "-q", "-m", "feat: next")
	if got, want := strings.Join(pathsOf(x.Files()), "\n"), r.Git("ls-tree", "-r", "--name-only", "HEAD"); got != want {
		t.Errorf("files = %q, and the next commit records %q", got, want)
	}
	for _, format := range []string{"sha1", "sha256"} {
		unborn := gittest.New(t)
		unborn.Git("init", "-q", "-b", "main", "--object-format="+format, "repo")
		unborn.Write("repo/e.txt", "")
		unborn.Write("repo/n.txt", "n\n")
		unborn.Write("repo/i.txt", "i\n")
		unborn.Git("-C", "repo", "add", "e.txt", "n.txt")
		unborn.Git("-C", "repo", "add", "-N", "i.txt")
		unborn.Remove("repo/i.txt")
		got := pathsOf(indexOf(t, filepath.Join(unborn.Root, "repo"), source.IndexOptions{}).Files())
		if !slices.Equal(got, []string{"e.txt", "n.txt"}) {
			t.Errorf("%s unborn files = %v, want e.txt and n.txt", format, got)
		}
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

// TestNewIndexRefusesAnIndexThatChangedWhileItWasRead stands a git on PATH
// that stages when the changes are asked for, as a process that writes the
// index between the listing and the changes would: a path the listing
// lacks and a new blob at a listed path are each ErrGit naming the path,
// never a snapshot that holds two states of the index at once.
func TestNewIndexRefusesAnIndexThatChangedWhileItWasRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a shell script stands in for git, and Windows runs none named git")
	}
	gitPath, lookErr := exec.LookPath("git")
	if lookErr != nil {
		t.Fatal(lookErr)
	}
	bin := t.TempDir()
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, staged := range []string{"late.txt", "b.txt"} {
		script := "#!/bin/sh\nif [ \"$1\" = diff-index ]; then '" + gitPath + "' add " + staged + " || exit 1; fi\nexec '" + gitPath + "' \"$@\"\n"
		if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		r := gittest.New(t)
		r.Commit("feat: base", map[string]string{"a.txt": "a\n", "b.txt": "b0\n"})
		r.StageOnly("b.txt", "b1\n", "b2\n")
		r.Write("late.txt", "late\n")
		repo, err := source.Discover(t.Context(), r.Root)
		if err != nil {
			t.Fatal(err)
		}
		_, err = source.NewIndex(t.Context(), repo, source.IndexOptions{})
		if !errors.Is(err, source.ErrGit) || !strings.Contains(err.Error(), "changed while it was read") || !strings.Contains(err.Error(), staged) {
			t.Errorf("%s staged between the two reads: %v", staged, err)
		}
	}
}
