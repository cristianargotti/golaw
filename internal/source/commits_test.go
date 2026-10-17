package source_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/source"
)

// gitAs runs one git command under dir as another committer, whose name,
// email and time env gives, after the identity every source test commits as.
func gitAs(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(),
		"GIT_AUTHOR_NAME=tester", "GIT_AUTHOR_EMAIL=tester@example.com",
		"GIT_COMMITTER_NAME=tester", "GIT_COMMITTER_EMAIL=tester@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

// mate is a teammate who committed on the first day of 2020.
func mate() []string {
	return []string{"GIT_COMMITTER_NAME=mate", "GIT_COMMITTER_EMAIL=mate@example.com", "GIT_COMMITTER_DATE=2020-01-01T00:00:00Z"}
}

// historyOfCommits is a repository whose history holds a teammate's root
// commit of 2020, a commit of two files, a rename, an empty commit and a
// merge of a branch of one commit, and the name of its root commit.
func historyOfCommits(t *testing.T) (string, string) {
	t.Helper()
	root := initRepo(t)
	write(t, root, "t.txt", "one\n")
	gitAs(t, root, mate(), "add", "t.txt")
	gitAs(t, root, mate(), "commit", "-q", "-m", "one")
	first := gitRun(t, root, "rev-parse", "HEAD")
	write(t, root, "a.txt", "two\n")
	write(t, root, "odd name \u00e9.txt", "three\n")
	commitAll(t, root, "two")
	gitRun(t, root, "mv", "a.txt", "b.txt")
	commitAll(t, root, "three")
	gitRun(t, root, "commit", "-q", "--allow-empty", "-m", "four")
	gitRun(t, root, "checkout", "-q", "-b", "side")
	write(t, root, "s.txt", "four\n")
	commitAll(t, root, "five")
	gitRun(t, root, "checkout", "-q", "main")
	gitRun(t, root, "merge", "-q", "--no-ff", "-m", "six", "side")
	return root, first
}

// TestCommitsNamesWhatEachCommitChanged asks git log once for the commits
// between two HEADs: each one not a merge, with its committer, the second it
// was committed and the paths it changed, a rename as both of its paths, an
// empty commit with none and the root commit, a teammate's of 2020, with
// every path it holds; every commit of HEAD when the earlier HEAD is
// unborn, none older than the second the walk is bounded by, and a
// directory outside git is an error.
func TestCommitsNamesWhatEachCommitChanged(t *testing.T) {
	t.Parallel()
	root, first := historyOfCommits(t)
	repo, err := source.Discover(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	all, err := source.Commits(t.Context(), repo, "", repo.Head, 0)
	if err != nil || len(all) != 5 {
		t.Fatalf("every commit = %+v, %v", all, err)
	}
	named := map[string][]string{}
	for _, commit := range all {
		named[strings.Join(commit.Paths, "|")] = append(named[strings.Join(commit.Paths, "|")], commit.Committer+" "+strconv.FormatInt(commit.Time, 10))
	}
	if !slices.Equal(named["t.txt"], []string{"mate <mate@example.com> 1577836800"}) || len(named["a.txt|b.txt"]) != 1 ||
		len(named[""]) != 1 || len(named["s.txt"]) != 1 || len(named["a.txt|odd name \u00e9.txt"]) != 1 {
		t.Fatalf("the paths of each commit = %v", named)
	}
	since, err := source.Commits(t.Context(), repo, first, repo.Head, 0)
	if err != nil || len(since) != 4 {
		t.Fatalf("the commits since the first = %+v, %v", since, err)
	}
	recent, err := source.Commits(t.Context(), repo, "", repo.Head, 1600000000)
	if err != nil || len(recent) != 4 || slices.ContainsFunc(recent, func(c source.Commit) bool { return strings.HasPrefix(c.Committer, "mate") }) {
		t.Fatalf("the commits of the walk bounded after 2020 = %+v, %v", recent, err)
	}
	if none, err := source.Commits(t.Context(), repo, repo.Head, repo.Head, 0); err != nil || len(none) != 0 {
		t.Fatalf("a HEAD against itself = %+v, %v", none, err)
	}
	if _, err := source.Commits(t.Context(), source.Repo{Root: t.TempDir()}, "", repo.Head, 0); err == nil {
		t.Fatal("a directory outside git answered commits")
	}
}

// TestCommitterIsTheIdentityGitCommitsAs answers the name and email git var
// writes for the committer of the repository, without the time after them,
// and an error where git cannot run.
func TestCommitterIsTheIdentityGitCommitsAs(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	gitRun(t, root, "config", "user.name", "the tester")
	gitRun(t, root, "config", "user.email", "tester@example.com")
	repo, err := source.Discover(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := source.Committer(t.Context(), repo); err != nil || got != "the tester <tester@example.com>" {
		t.Fatalf("Committer = %q, %v", got, err)
	}
	if _, err := source.Committer(t.Context(), source.Repo{Root: filepath.Join(root, "absent")}); err == nil {
		t.Fatal("a directory git cannot run in answered a committer")
	}
}
