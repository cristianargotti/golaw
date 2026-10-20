package gittest_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
)

// fatal records a failure instead of ending the test, so a test proves that
// a helper fails the test it belongs to.
type fatal struct {
	testing.TB

	failed bool
}

func (f *fatal) Fatalf(string, ...any) { f.failed = true }

func TestNewIsAnEmptyRepositoryOnMain(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	if r.Head() != "" {
		t.Errorf("an unborn branch has no HEAD, got %q", r.Head())
	}
	if got := r.Git("symbolic-ref", "--short", "HEAD"); got != "main" {
		t.Errorf("branch = %q, want main", got)
	}
	if resolved, err := filepath.EvalSymlinks(r.Root); err != nil || resolved != r.Root {
		t.Errorf("root %s is not a real path: %s, %v", r.Root, resolved, err)
	}
	if dir := r.GitDir(); !filepath.IsAbs(dir) || filepath.Base(dir) != ".git" {
		t.Errorf("git directory = %s", dir)
	}
}

func TestIsolatedKeepsGitFromItsSurroundings(t *testing.T) {
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "elsewhere"))
	environment := gittest.Isolated()
	for _, entry := range environment {
		if strings.HasPrefix(strings.ToUpper(entry), "GIT_DIR=") {
			t.Errorf("Isolated keeps %s", entry)
		}
	}
	for _, want := range []string{"GIT_AUTHOR_NAME=" + gittest.Name, "GIT_COMMITTER_EMAIL=" + gittest.Email, "GIT_CONFIG_NOSYSTEM=1"} {
		if !slices.Contains(environment, want) {
			t.Errorf("Isolated lacks %s", want)
		}
	}
	r := gittest.New(t)
	r.Commit("feat: one", map[string]string{"a.txt": "a\n"})
	if got := r.Git("log", "--format=%an <%ae>|%cn <%ce>"); got != "tester <tester@example.com>|tester <tester@example.com>" {
		t.Errorf("identity = %q", got)
	}
}

func TestCommitAndTheStagedStates(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	first := r.Commit("feat: base", map[string]string{"a.txt": "one\n", "b.txt": "b\n", "c.txt": "c\n"})
	if r.Head() != first || len(first) != 40 {
		t.Fatalf("HEAD %s, commit %s", r.Head(), first)
	}
	r.StageOnly("a.txt", "two\n", "three\n")
	r.StageDeletion("b.txt")
	r.StageRename("c.txt", "dir/c.txt")
	r.Stage(map[string]string{"e.txt": "e\n"})
	if got := r.Git("show", ":a.txt"); got != "two" {
		t.Errorf("staged a.txt = %q, want two", got)
	}
	if data, err := os.ReadFile(filepath.Join(r.Root, "a.txt")); err != nil || string(data) != "three\n" {
		t.Errorf("worktree a.txt = %q, %v", data, err)
	}
	status := r.Git("status", "--porcelain")
	for _, want := range []string{"MM a.txt", "D  b.txt", "R  c.txt -> dir/c.txt", "A  e.txt"} {
		if !strings.Contains(status, want) {
			t.Errorf("status %q lacks %q", status, want)
		}
	}
	r.Write("d.txt", "d\n")
	r.Remove("d.txt")
	if _, err := os.Stat(filepath.Join(r.Root, "d.txt")); !os.IsNotExist(err) {
		t.Errorf("d.txt after Remove: %v", err)
	}
}

func TestBranchAndAMergeInProgress(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Commit("feat: base", map[string]string{"shared.md": "base\n"})
	tip := r.Branch("side", map[string]string{"shared.md": "theirs\n"})
	if r.Git("symbolic-ref", "--short", "HEAD") != "main" || r.Git("rev-parse", "side") != tip {
		t.Fatalf("Branch left %s, side %s", r.Git("symbolic-ref", "--short", "HEAD"), tip)
	}
	r.Commit("feat: ours", map[string]string{"shared.md": "ours\n"})
	conflict := r.Worktree("conflict")
	conflict.MergeInProgress("side")
	if conflict.Git("rev-parse", "MERGE_HEAD") != tip || conflict.Git("ls-files", "-u") == "" {
		t.Error("a conflicting merge stays in progress with the path unmerged")
	}
	r.MergeInProgress("side", "-X", "theirs")
	if r.Git("rev-parse", "MERGE_HEAD") != tip || r.Git("show", ":shared.md") != "theirs" {
		t.Error("a strategy option resolves the merge the way the command asks")
	}
}

func TestWorktreeAndAnAlternateIndex(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Commit("feat: base", map[string]string{"a.txt": "a\n"})
	w := r.Worktree("feature")
	if w.Root == r.Root || w.Git("symbolic-ref", "--short", "HEAD") != "feature" || w.GitDir() == r.GitDir() {
		t.Errorf("worktree %s on %s with %s", w.Root, w.Git("symbolic-ref", "--short", "HEAD"), w.GitDir())
	}
	index := r.AlternateIndex()
	alternate := []string{"GIT_INDEX_FILE=" + index}
	if got := r.GitEnv(alternate, "ls-files"); got != "a.txt" {
		t.Errorf("alternate index lists %q", got)
	}
	r.GitEnv(alternate, "rm", "-q", "--cached", "a.txt")
	if r.GitEnv(alternate, "ls-files") != "" || r.Git("ls-files") != "a.txt" {
		t.Error("the alternate index changes apart from the index of the repository")
	}
}

func TestHelpersFailTheTestTheyBelongTo(t *testing.T) {
	t.Parallel()
	f := &fatal{TB: t}
	r := gittest.New(f)
	r.Git("no-such-command")
	if !f.failed {
		t.Error("a failing git command fails the test")
	}
	f.failed = false
	r.MergeInProgress("no-such-branch")
	if !f.failed {
		t.Error("a merge that never starts fails the test")
	}
	f.failed = false
	r.Remove("absent.txt")
	if !f.failed {
		t.Error("removing an absent file fails the test")
	}
}
