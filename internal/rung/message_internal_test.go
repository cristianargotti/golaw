package rung

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

func TestAuthorIsTheIdentityWithoutItsTime(t *testing.T) {
	t.Parallel()
	r := repository(t, "")
	if got := author(t.Context(), Dependencies{Dir: r.Root, Environ: gittest.Isolated()}); got != "tester <tester@example.com>" {
		t.Errorf("author = %q", got)
	}
	if got := author(t.Context(), Dependencies{Dir: filepath.Join(r.Root, "absent"), Environ: gittest.Isolated()}); got != "" {
		t.Errorf("an author git cannot name = %q", got)
	}
}

func TestReadCommitReadsTheLastCommentSetting(t *testing.T) {
	t.Parallel()
	r := repository(t, "")
	r.Write("message.txt", "fix: keep it\n")
	d := Dependencies{Dir: r.Root, Environ: gittest.Isolated()}
	r.Git("config", "core.commentChar", ";")
	r.Git("config", "core.commentString", "//")
	commit, err := readCommit(t.Context(), d, "message.txt")
	if err != nil || commit.CommentChar != "//" || commit.Cleanup != model.CleanupStrip || commit.Message != "fix: keep it\n" {
		t.Fatalf("commit = %+v, %v", commit, err)
	}
	r.Git("config", "--unset", "core.commentString")
	r.Git("config", "core.commentChar", "auto")
	if commit, err := readCommit(t.Context(), d, filepath.Join(r.Root, "message.txt")); err != nil || commit.CommentChar != "" {
		t.Errorf("auto reads as the default: %+v, %v", commit, err)
	}
	r.Write(".git/config", "[core\n")
	if _, err := readCommit(t.Context(), d, "message.txt"); !errors.Is(err, source.ErrGit) {
		t.Errorf("a configuration git refuses: %v", err)
	}
	if _, err := gitSettings(t.Context(), Dependencies{Dir: filepath.Join(r.Root, "absent")}); err == nil {
		t.Error("a git that cannot start is an error")
	}
}

func TestIndexFileIsAbsoluteAgainstTheWorktree(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	absolute := filepath.Join(root, "index")
	cases := map[string][]string{
		filepath.Join(root, ".git", "index.lock"): {"git_index_file=.git/index.lock"},
		absolute: {"GIT_INDEX_FILE=" + filepath.Join(root, "other"), "GIT_INDEX_FILE=" + absolute},
		"":       {"GIT_INDEX=x"},
	}
	for want, environ := range cases {
		if got := indexFile(Dependencies{Dir: root, Environ: environ}); got != want {
			t.Errorf("indexFile(%v) = %q, want %q", environ, got, want)
		}
	}
}
