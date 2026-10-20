package runner_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
)

func TestMaterializeUsesGitBytes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		mode string
		want string
	}{
		{"red-opposite-worktree", "index", "staged\n"},
		{"near-revision", "revision", "committed\n"},
		{"near-alternate-index", "alternate", "alternate\n"},
		{"near-converted-line-endings", "converted", "staged\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			base := repo.Commit("chore: seed", map[string]string{
				"file.txt": "committed\n", "keep.txt": "keep\n",
			})
			repo.StageOnly("file.txt", "staged\n", "dirty\n")
			requireGitBytes(t, repo, gitBytesInput(t, repo, base, tc.mode), tc.want)
		})
	}
}

// gitBytesInput is the materialization a row asks for: the index, the
// revision of the seed commit, the index under converted line endings, or
// an alternate index that holds other bytes.
func gitBytesInput(t *testing.T, repo *gittest.Repo, base, mode string) runner.MaterializeInput {
	t.Helper()
	in := runner.MaterializeInput{
		Dir: repo.Root, Env: gittest.Isolated(),
		Parent: t.TempDir(), Bounds: runner.DefaultBounds(),
	}
	switch mode {
	case "revision":
		in.Rev = base
	case "converted":
		in.Env = append(in.Env, "GIT_CONFIG_VALUE_0=true")
	case "alternate":
		index := repo.AlternateIndex()
		repo.Write("file.txt", "alternate\n")
		repo.GitEnv([]string{"GIT_INDEX_FILE=" + index},
			"add", "--", "file.txt")
		repo.Write("file.txt", "dirty\n")
		in.Env = append(in.Env,
			"GIT_DIR="+repo.GitDir(), "GIT_INDEX_FILE="+index)
	}
	return in
}

// requireGitBytes requires the tree of a materialization to hold the want
// bytes and no git metadata, to leave the repository unchanged, and to go
// away on every close.
func requireGitBytes(t *testing.T, repo *gittest.Repo, in runner.MaterializeInput, want string) {
	t.Helper()
	before := repo.Git("status", "--porcelain=v1")
	tree, err := runner.Materialize(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(tree.Dir, "file.txt"))
	if err != nil || string(data) != want {
		t.Fatalf("content = %q, %v", data, err)
	}
	if after := repo.Git("status", "--porcelain=v1"); after != before {
		t.Fatalf("repository changed: %q -> %q", before, after)
	}
	_, err = os.Stat(filepath.Join(tree.Dir, ".git"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("git metadata materialized: %v", err)
	}
	for range 2 {
		if closeErr := tree.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	_, err = os.Stat(tree.Dir)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("tree remains: %v", err)
	}
}

func TestMaterializeBounds(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		bounds runner.Bounds
		fail   bool
	}{
		{"red-bytes", runner.Bounds{Bytes: 3, Files: 2}, true},
		{"near-exact-bytes", runner.Bounds{Bytes: 4, Files: 2}, false},
		{"red-files", runner.Bounds{Bytes: 4, Files: 1}, true},
		{"near-exact-files", runner.Bounds{Bytes: 4, Files: 2}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			repo.Commit("chore: seed", map[string]string{
				"a.txt": "aa", "b.txt": "bb",
			})
			parent := t.TempDir()
			tree, err := runner.Materialize(t.Context(), runner.MaterializeInput{
				Dir: repo.Root, Env: gittest.Isolated(),
				Parent: parent, Bounds: tc.bounds,
			})
			requireBoundResult(t, tree, err, tc.fail)
			entries, err := os.ReadDir(parent)
			if err != nil || len(entries) != 0 {
				t.Fatalf("resources remain = %v, %v", entries, err)
			}
		})
	}
}

// requireBoundResult requires a materialization over its bounds to answer
// ErrBound and no tree, and one within them a tree that closes.
func requireBoundResult(t *testing.T, tree *runner.Tree, err error, fail bool) {
	t.Helper()
	if fail {
		if !errors.Is(err, runner.ErrBound) || tree != nil {
			t.Fatalf("bound result = %v, %v", tree, err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if closeErr := tree.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
}

func TestMaterializeKeepsLargeBlobs(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	content := strings.Repeat("x", runner.MaxOutputBytes+1)
	repo.Commit("chore: seed", map[string]string{"large.txt": content})
	tree, err := runner.Materialize(t.Context(), runner.MaterializeInput{
		Dir: repo.Root, Env: gittest.Isolated(), Parent: t.TempDir(),
		Bounds: runner.Bounds{Bytes: int64(len(content)), Files: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(tree.Dir, "large.txt"))
	if err != nil || string(data) != content {
		t.Fatalf("large blob bytes = %d, %v", len(data), err)
	}
	if err := tree.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMaterializeFailureLeavesNoTree(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"near-gitlink", "cancelled", "inside-repository"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			repo.Commit("chore: seed", map[string]string{"keep.txt": "keep\n"})
			ctx, parent := failureSetting(t, repo, kind)
			tree, err := runner.Materialize(ctx, runner.MaterializeInput{
				Dir: repo.Root, Env: gittest.Isolated(),
				Parent: parent, Bounds: runner.DefaultBounds(),
			})
			if kind == "near-gitlink" {
				requireEmptyGitlink(t, tree, err)
			} else {
				requireNoTree(t, kind, tree, err)
			}
			requireNoResources(t, parent)
		})
	}
}

// failureSetting is the context and the parent directory of a failure row:
// a staged gitlink, a context already canceled, or a parent inside the
// repository.
func failureSetting(t *testing.T, repo *gittest.Repo, kind string) (context.Context, string) {
	t.Helper()
	switch kind {
	case "near-gitlink":
		repo.Git("update-index", "--add", "--cacheinfo",
			"160000", strings.Repeat("f", 40), "external")
	case "cancelled":
		cancelled, cancel := context.WithCancel(t.Context())
		cancel()
		return cancelled, t.TempDir()
	case "inside-repository":
		return t.Context(), filepath.Join(repo.Root, "trees")
	}
	return t.Context(), t.TempDir()
}

// requireEmptyGitlink requires a staged gitlink to materialize as an empty
// directory.
func requireEmptyGitlink(t *testing.T, tree *runner.Tree, err error) {
	t.Helper()
	if err != nil || tree == nil {
		t.Fatalf("gitlink result = %v, %v", tree, err)
	}
	entries, readErr := os.ReadDir(filepath.Join(tree.Dir, "external"))
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("gitlink directory = %v, %v", entries, readErr)
	}
	if closeErr := tree.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
}

// requireNoTree requires a failed materialization to answer an error and
// no tree, and the cancellation of the context for the canceled row.
func requireNoTree(t *testing.T, kind string, tree *runner.Tree, err error) {
	t.Helper()
	if err == nil || tree != nil {
		t.Fatalf("result = %v, %v", tree, err)
	}
	if kind == "cancelled" && !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
}

// requireNoResources requires the parent directory of the trees to hold
// nothing, or to be absent.
func requireNoResources(t *testing.T, parent string) {
	t.Helper()
	entries, readErr := os.ReadDir(parent)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("resources remain = %v", entries)
	}
}
