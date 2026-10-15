package source_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/source"
)

func TestDiscover(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	write(t, root, "a.txt", "a\n")
	head := commitAll(t, root, "one")
	repo, err := source.Discover(t.Context(), filepath.Join(root, "sub", "..", "."))
	if err != nil {
		t.Fatal(err)
	}
	if repo.Root != root || repo.Head != head || repo.GitDir != filepath.Join(root, ".git") {
		t.Errorf("Discover = %+v", repo)
	}
	if repo.Identity() != filepath.ToSlash(root) {
		t.Errorf("Identity = %q", repo.Identity())
	}
	gitRun(t, root, "config", "core.ignorecase", "false")
	if again, err := source.Discover(t.Context(), root); err != nil || again.IgnoreCase {
		t.Errorf("ignorecase false: %+v, %v", again, err)
	}
	gitRun(t, root, "config", "core.ignorecase", "true")
	if again, err := source.Discover(t.Context(), root); err != nil || !again.IgnoreCase {
		t.Errorf("ignorecase true: %+v, %v", again, err)
	}
}

func TestDiscoverUnbornAndSubdirectory(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	write(t, root, "dir/a.txt", "a\n")
	repo, err := source.Discover(t.Context(), filepath.Join(root, "dir"))
	if err != nil {
		t.Fatal(err)
	}
	if repo.Root != root || repo.Head != "" {
		t.Errorf("unborn from a subdirectory = %+v", repo)
	}
}

func TestDiscoverLinkedWorktree(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	write(t, root, "a.txt", "a\n")
	commitAll(t, root, "one")
	linked := filepath.Join(t.TempDir(), "linked")
	gitRun(t, root, "worktree", "add", "-q", linked, "-b", "linked")
	repo, err := source.Discover(t.Context(), linked)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(linked)
	if err != nil {
		t.Fatal(err)
	}
	if repo.Root != resolved || repo.GitDir != filepath.Join(root, ".git", "worktrees", "linked") {
		t.Errorf("linked worktree = %+v", repo)
	}
}

func TestDiscoverOutcomes(t *testing.T) {
	t.Parallel()
	if _, err := source.Discover(t.Context(), t.TempDir()); !errors.Is(err, source.ErrNotRepository) {
		t.Errorf("outside git: %v, want ErrNotRepository", err)
	}
	if _, err := source.Discover(t.Context(), filepath.Join(t.TempDir(), "absent")); !errors.Is(err, source.ErrGit) {
		t.Errorf("absent directory: %v, want ErrGit", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := source.Discover(ctx, initRepo(t))
	if !errors.Is(err, source.ErrGit) || errors.Is(err, source.ErrNotRepository) || !strings.HasPrefix(err.Error(), "git rev-parse") {
		t.Errorf("canceled context: %v, want ErrGit naming the command", err)
	}
}
