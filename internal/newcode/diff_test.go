package newcode_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/newcode"
)

func gitContext(t *testing.T) context.Context {
	t.Helper()
	return newcode.WithEnviron(t.Context(), gittest.Isolated())
}

// history is main with a base branch, a side branch that moved on from the
// base, and one change on main that edits, adds, moves and deletes Go
// files and edits a file that is not Go.
func history(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.New(t)
	repo.Commit("feat: the base", map[string]string{
		"go.mod":    "module example.test/m\n\ngo 1.27\n",
		"a.go":      "package m\n\nfunc A() int {\n\treturn 1\n}\n",
		"moved.go":  "package m\n\nfunc M() {}\n",
		"gone.go":   "package m\n",
		"notes.txt": "first\n",
	})
	repo.Git("branch", "base")
	repo.Branch("side", map[string]string{"side.go": "package m\n\nfunc S() {}\n"})
	repo.Git("rm", "-q", "gone.go")
	repo.Git("mv", "moved.go", "moved_now.go")
	repo.Commit("feat: change the package", map[string]string{
		"a.go":          "package m\n\nfunc A() int {\n\treturn 2\n}\n\n// B is new.\nvar B = 3\n",
		"pkg/ação x.go": "package pkg\n\nfunc P() {}\n",
		"notes.txt":     "second\n",
	})
	return repo
}

func changedSinceBase() map[string][]int {
	return map[string][]int{"a.go": {4, 6, 7, 8}, "moved_now.go": {1, 2, 3}, "pkg/ação x.go": {1, 2, 3}}
}

func TestNewLinesNameWhatHeadAddsSinceTheMergeBase(t *testing.T) {
	t.Parallel()
	repo := history(t)
	for _, tc := range []struct {
		name string
		base string
		want map[string][]int
	}{
		{"red-the-base-branch", "base", changedSinceBase()},
		{"near-a-branch-that-moved-on-counts-from-the-fork", "side", changedSinceBase()},
		{"near-head-adds-nothing", "HEAD", map[string][]int{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := newcode.NewLines(gitContext(t), repo.Root, tc.base)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("NewLines(%s) = %v, %v, want %v", tc.base, got, err, tc.want)
			}
		})
	}
}

func TestMergeBaseIsTheForkOfTheBase(t *testing.T) {
	t.Parallel()
	repo := history(t)
	got, err := newcode.MergeBase(gitContext(t), repo.Root, "side")
	if want := repo.Git("rev-parse", "base"); err != nil || got != want {
		t.Fatalf("MergeBase = %q, %v, want %q", got, err, want)
	}
}

func TestNewLinesRefuseABaseTheyCannotResolve(t *testing.T) {
	t.Parallel()
	repo := history(t)
	repo.Git("checkout", "-q", "--orphan", "lonely")
	repo.Commit("feat: an unrelated root", map[string]string{"lonely.go": "package lonely\n"})
	repo.Git("checkout", "-q", "main")
	for _, tc := range []struct {
		name string
		root string
		base string
		want error
	}{
		{"red-an-unknown-ref", repo.Root, "nowhere", newcode.ErrBase},
		{"red-an-option-shaped-ref", repo.Root, "--output=x", newcode.ErrBase},
		{"red-an-unrelated-history", repo.Root, "lonely", newcode.ErrBase},
		{"red-no-repository", t.TempDir(), "main", newcode.ErrDiff},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := newcode.NewLines(gitContext(t), tc.root, tc.base)
			if !errors.Is(err, tc.want) || got != nil {
				t.Fatalf("NewLines(%s) = %v, %v, want %v", tc.base, got, err, tc.want)
			}
		})
	}
}

func TestLastTagIsTheNearestTagOfHead(t *testing.T) {
	t.Parallel()
	repo := history(t)
	ctx := gitContext(t)
	if _, err := newcode.LastTag(ctx, repo.Root); !errors.Is(err, newcode.ErrBase) {
		t.Fatalf("no tag = %v, want ErrBase", err)
	}
	repo.Git("tag", "v0.1.0", "base")
	tag, err := newcode.LastTag(ctx, repo.Root)
	if err != nil || tag != "v0.1.0" {
		t.Fatalf("LastTag = %q, %v, want v0.1.0", tag, err)
	}
	lines, err := newcode.NewLines(ctx, repo.Root, tag)
	if err != nil || !reflect.DeepEqual(lines, changedSinceBase()) {
		t.Fatalf("since v0.1.0 = %v, %v", lines, err)
	}
	repo.Git("tag", "-a", "-m", "release v0.2.0", "v0.2.0")
	if tag, err = newcode.LastTag(ctx, repo.Root); err != nil || tag != "v0.2.0" {
		t.Fatalf("LastTag = %q, %v, want the annotated v0.2.0 on HEAD", tag, err)
	}
}

func TestDefaultBranchIsTheRemoteHead(t *testing.T) {
	t.Parallel()
	repo := history(t)
	ctx := gitContext(t)
	if branch, err := newcode.DefaultBranch(ctx, repo.Root); err != nil || branch != "" {
		t.Fatalf("no remote = %q, %v, want empty", branch, err)
	}
	repo.Git("update-ref", "refs/remotes/origin/main", "base")
	repo.Git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	branch, err := newcode.DefaultBranch(ctx, repo.Root)
	if err != nil || branch != "refs/remotes/origin/main" {
		t.Fatalf("DefaultBranch = %q, %v", branch, err)
	}
	if lines, err := newcode.NewLines(ctx, repo.Root, branch); err != nil || !reflect.DeepEqual(lines, changedSinceBase()) {
		t.Fatalf("since the default branch = %v, %v", lines, err)
	}
	if _, err := newcode.DefaultBranch(ctx, t.TempDir()); !errors.Is(err, newcode.ErrDiff) {
		t.Fatalf("outside a repository = %v, want ErrDiff", err)
	}
}

// TestDefaultBranchWithoutTheRemoteHead is the checkout a CI runner leaves:
// the remote branches fetched and refs/remotes/origin/HEAD never set.
func TestDefaultBranchWithoutTheRemoteHead(t *testing.T) {
	t.Parallel()
	repo := history(t)
	ctx := gitContext(t)
	repo.Git("update-ref", "refs/remotes/origin/master", "base")
	if branch, err := newcode.DefaultBranch(ctx, repo.Root); err != nil || branch != "refs/remotes/origin/master" {
		t.Fatalf("origin/master alone = %q, %v", branch, err)
	}
	repo.Git("update-ref", "refs/remotes/origin/main", "base")
	repo.Git("update-ref", "refs/remotes/origin/main-old", "base")
	if branch, err := newcode.DefaultBranch(ctx, repo.Root); err != nil || branch != "refs/remotes/origin/main" {
		t.Fatalf("origin/main beside origin/master = %q, %v, want main first", branch, err)
	}
}
