package source_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

// revisionOf freezes the tree a revision of the repository at root names.
func revisionOf(t *testing.T, root, rev string) *source.Revision {
	t.Helper()
	repo, err := source.Discover(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := source.NewRevision(t.Context(), repo, rev)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestRevisionReadsTheTreeOfACommitNeverTheIndex(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	first := r.Commit("feat: base", map[string]string{"a.txt": "one\n", "dir/b_test.go": "package b\n"})
	r.Commit("feat: second", map[string]string{"a.txt": "two\n", "c.txt": "c\n"})
	r.StageOnly("a.txt", "staged\n", "worktree\n")
	at := revisionOf(t, r.Root, first)
	if at.Identity() != "tree:"+first {
		t.Errorf("identity = %s", at.Identity())
	}
	if got := pathsOf(at.Files()); !slices.Equal(got, []string{"a.txt", "dir/b_test.go"}) {
		t.Errorf("files = %v", got)
	}
	if got := readText(t, at, "a.txt"); got != "one\n" || at.Class("dir/b_test.go") != model.ClassTest {
		t.Errorf("a.txt = %q, dir/b_test.go is %s", got, at.Class("dir/b_test.go"))
	}
	if _, err := at.Read("c.txt"); !errors.Is(err, model.ErrNotInSnapshot) {
		t.Errorf("c.txt at the first commit: %v", err)
	}
	head := revisionOf(t, r.Root, "HEAD")
	if head.Identity() != "tree:"+r.Head() || readText(t, head, "a.txt") != "two\n" {
		t.Errorf("HEAD is %s and holds %q", head.Identity(), readText(t, head, "a.txt"))
	}
	if got, err := head.Blob(r.Git("rev-parse", first+":a.txt")); err != nil || string(got) != "one\n" {
		t.Errorf("an older blob = %q, %v", got, err)
	}
	if !head.Any(func(p string) bool { return p == "c.txt" }) || len(head.Select(func(p string) bool { return strings.HasPrefix(p, "dir/") })) != 1 {
		t.Error("Select and Any answer the paths of the tree")
	}
}

func TestRevisionOfTheEmptyTree(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	empty := revisionOf(t, r.Root, "")
	if empty.Identity() != "tree:empty" || len(empty.Files()) != 0 || empty.Class("a.txt") != model.ClassUnknown {
		t.Errorf("empty tree = %s, %v", empty.Identity(), empty.Files())
	}
	if _, err := empty.Read("a.txt"); !errors.Is(err, model.ErrNotInSnapshot) {
		t.Errorf("read = %v, want ErrNotInSnapshot", err)
	}
}

func TestRevisionClassifiesLinksAndSubmodules(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Stage(map[string]string{"target.txt": "target.txt"})
	link := r.Git("rev-parse", ":target.txt")
	r.Git("update-index", "--add", "--cacheinfo", "120000,"+link+",link")
	r.Git("update-index", "--add", "--cacheinfo", "160000,"+strings.Repeat("1", 40)+",sub")
	r.Commit("feat: links", nil)
	head := revisionOf(t, r.Root, "HEAD")
	for rel, want := range map[string]model.FileClass{"link": model.ClassSymlink, "sub": model.ClassGitlink, "target.txt": model.ClassTracked} {
		if got := head.Class(rel); got != want {
			t.Errorf("%s = %s, want %s", rel, got, want)
		}
	}
}

func TestNewRevisionRefusesANameThatIsNoCommit(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Commit("feat: base", map[string]string{"a.txt": "a\n"})
	repo, err := source.Discover(t.Context(), r.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, rev := range []string{"no-such-revision", "--all", "HEAD:a.txt"} {
		if _, err := source.NewRevision(t.Context(), repo, rev); !errors.Is(err, source.ErrGit) {
			t.Errorf("%s: %v, want ErrGit", rev, err)
		}
	}
	if _, err := source.NewRevision(t.Context(), source.Repo{Root: t.TempDir()}, "HEAD"); !errors.Is(err, source.ErrGit) {
		t.Errorf("outside git: %v, want ErrGit", err)
	}
}
