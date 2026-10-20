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

func TestPickedReadsTheCommentCharacterGitPicked(t *testing.T) {
	t.Parallel()
	verbose := "Merge branch 'side'\n\n# Conflicts:\n;\n; ------------------------ >8 ------------------------\n; Do not modify\n+diff\n"
	mixed := "fix: keep it\n\n# kept or not\n; kept or not\nthe body\n"
	cases := []struct {
		name, message, comment, judged string
		strip                          bool
	}{
		{"the scissors line of a verbose merge", verbose, ";", verbose, true},
		{"a scissors line the person wrote above git's", "fix: x\n! ------------------------ >8 ------------------------\n" + verbose, "!", "fix: x\n! ------------------------ >8 ------------------------\n" + verbose, true},
		{"no line a candidate starts", "fix: keep it\n\nthe body\n", "", "fix: keep it\n\nthe body\n", true},
		{"one candidate", "fix: keep it\n\n; a comment\n", ";", "fix: keep it\n\n; a comment\n", true},
		{"two candidates under strip", mixed, "", "fix: keep it\n\nthe body\n", true},
		{"two candidates git keeps", mixed, "#", mixed, false},
	}
	for _, c := range cases {
		if comment, judged := picked(c.message, c.strip); comment != c.comment || judged != c.judged {
			t.Errorf("%s: picked = %q, %q, want %q, %q", c.name, comment, judged, c.comment, c.judged)
		}
	}
	r := repository(t, "")
	r.Write("message.txt", "fix: keep it\n\n; a comment\n")
	r.Git("config", "core.commentChar", "AUTO")
	if commit, err := readCommit(t.Context(), Dependencies{Dir: r.Root, Environ: gittest.Isolated()}, "message.txt"); err != nil || commit.CommentChar != ";" {
		t.Errorf("auto in capitals, as git reads it = %+v, %v", commit, err)
	}
}
