package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
)

// The moments of the commits of the fixtures: a teammate's commit is older
// than the clock of the test binding, as the commits a pull brings are, or
// lies in the future of a skewed clock; earlier is also a commit the
// developer made before the command.
const (
	earlier = "2026-10-01T09:00:00Z"
	future  = "2030-01-01T00:00:00Z"
)

// gitAs runs one git command in a directory with the environment of a commit
// another person or another moment made, and fails the test on an error.
func gitAs(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// at is the environment of a commit made at a moment by the identity of the
// repository.
func at(moment string) []string {
	return []string{"GIT_AUTHOR_DATE=" + moment, "GIT_COMMITTER_DATE=" + moment}
}

// mate is the environment of a commit a teammate made at a moment.
func mate(moment string) []string {
	return append(at(moment), "GIT_AUTHOR_NAME=mate", "GIT_AUTHOR_EMAIL=mate@example.test",
		"GIT_COMMITTER_NAME=mate", "GIT_COMMITTER_EMAIL=mate@example.test")
}

// commitAs writes files and commits every change with an environment.
func commitAs(t *testing.T, root string, env []string, files map[string]string) {
	t.Helper()
	writeFiles(t, root, files)
	gitAs(t, root, env, "add", "-A")
	gitAs(t, root, env, "commit", "-q", "-m", "docs: the notes")
}

// sideBranch commits files on a new branch named side with an environment
// and checks the branch the repository was on out again.
func sideBranch(t *testing.T, root string, env []string, files map[string]string) {
	t.Helper()
	gitRun(t, root, "checkout", "-q", "-b", "side")
	commitAs(t, root, env, files)
	gitRun(t, root, "checkout", "-q", "-")
}

// pushedByMate gives the repository a bare remote named origin holding its
// branch, onto which a teammate pushed a commit of files at a moment.
func pushedByMate(t *testing.T, root string, files map[string]string) {
	t.Helper()
	remote, clone := filepath.Join(t.TempDir(), "remote.git"), filepath.Join(t.TempDir(), "clone")
	gitRun(t, root, "init", "-q", "--bare", remote)
	gitRun(t, root, "remote", "add", "origin", remote)
	gitRun(t, root, "push", "-q", "origin", "HEAD:refs/heads/main")
	gitRun(t, root, "clone", "-q", "--branch", "main", remote, clone)
	commitAs(t, clone, mate(earlier), files)
	gitRun(t, clone, "push", "-q", "origin", "HEAD:refs/heads/main")
}

// moveCase is a command that moves HEAD: what the repository holds before
// its pre event, what it does, the file a finding names when the post event
// feeds the dash back, and the file the notice names as brought by the move.
type moveCase struct {
	name           string
	before, during func(root string)
	judged, named  string
}

// commandMoves are commands that move HEAD over commits: the red fixtures,
// a commit the command created holding a dash, whether it replays a local
// commit, squashes a teammate's branch or picks a teammate's commit, and the
// near misses, a move over a teammate's commits alone, whatever brings it.
func commandMoves(t *testing.T) []moveCase {
	t.Helper()
	dash := "one \u2014 two\n"
	theirs := func(env []string) func(string) {
		return func(root string) { sideBranch(t, root, env, map[string]string{"theirs.md": dash}) }
	}
	return []moveCase{
		{"a local commit rebased onto a teammate's", theirs(mate(earlier)), func(root string) {
			commitAs(t, root, nil, map[string]string{"mine.md": dash})
			gitRun(t, root, "rebase", "-q", "side")
		}, "mine.md", "theirs.md"},
		{"a rebase that replays a local commit", func(root string) {
			gitRun(t, root, "branch", "-m", "trunk")
			sideBranch(t, root, at(earlier), map[string]string{"mine.md": dash})
			commitAs(t, root, mate(earlier), map[string]string{"other.md": "clean\n"})
		}, func(root string) { gitRun(t, root, "rebase", "-q", "trunk", "side") }, "mine.md", ""},
		{"a merge --squash then a commit", theirs(mate(earlier)), func(root string) {
			gitRun(t, root, "merge", "-q", "--squash", "side")
			gitRun(t, root, "commit", "-q", "-m", "docs: squash the notes")
		}, "theirs.md", ""},
		{"a cherry-pick of a teammate's commit", theirs(mate(earlier)), func(root string) {
			gitRun(t, root, "cherry-pick", "side")
		}, "theirs.md", ""},
		{"a fast-forward pull of a teammate's commit", func(root string) { pushedByMate(t, root, map[string]string{"theirs.md": dash}) }, func(root string) {
			gitRun(t, root, "pull", "-q", "--ff-only", "origin", "main")
		}, "", "theirs.md"},
		{"a checkout of a teammate's branch", theirs(mate(earlier)), func(root string) { gitRun(t, root, "checkout", "-q", "side") }, "", "theirs.md"},
		{"a fast-forward merge", theirs(mate(earlier)), func(root string) { gitRun(t, root, "merge", "-q", "--ff-only", "side") }, "", "theirs.md"},
		{"a merge commit", theirs(mate(earlier)), func(root string) {
			gitRun(t, root, "merge", "-q", "--no-ff", "-m", "build: merge the notes", "side")
		}, "", "theirs.md"},
		{"a reset --hard", theirs(mate(earlier)), func(root string) { gitRun(t, root, "reset", "-q", "--hard", "side") }, "", "theirs.md"},
		{"a teammate's commit of a future time", theirs(mate(future)), func(root string) { gitRun(t, root, "checkout", "-q", "side") }, "", "theirs.md"},
	}
}

// TestHookPostJudgesOnlyWhatTheCommandCommitted holds the cases of a
// command that moves HEAD through the real binding: of the files the move
// changed, those a commit the command created changed are judged, and every
// other one is named in a notice, in the configured language for the
// person, that asks for no fix and leaves it to the pipeline.
func TestHookPostJudgesOnlyWhatTheCommandCommitted(t *testing.T) {
	t.Parallel()
	for _, tc := range commandMoves(t) {
		e := enrolledHook(t)
		commitFiles(t, e.root, map[string]string{"legacy.md": "one \u2014 two\n"})
		tc.before(e.root)
		said := aroundCommand(t, e, tc.name, tc.during)
		agentText, humanText := "", ""
		if !fedBack(said) {
			agentText, humanText = noticeOf(t, hook.Reply{Stdout: []byte(said)})
		}
		judged := tc.judged != "" && fedBack(said) && strings.Contains(said, "("+tc.judged+":1)")
		named := tc.named == "" || strings.Contains(said, "brought from commits this command did not create, which ask for no fix: "+tc.named)
		if judged != (tc.judged != "") || !named || strings.Contains(said, "legacy.md") ||
			tc.judged == "" && (!strings.Contains(agentText, "the pipeline judges them") || !strings.Contains(humanText, "golaw n\u00e3o julgou os arquivos")) {
			t.Errorf("%s: post said %q", tc.name, said)
		}
	}
}

// TestHookBindingNamesTheCommitsAndTheCommitter reads, through the binding
// of the hook, the commits since a HEAD and the identity git commits as,
// and answers an error for a directory outside git and for a HEAD git does
// not hold.
func TestHookBindingNamesTheCommitsAndTheCommitter(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	h := hookBindingFor(t, b)
	root := repository(t, true)
	repo := hook.Repository{Root: root}
	commits, err := h.commits(t.Context(), repo, "", "HEAD", 0)
	if err != nil || len(commits) != 1 || commits[0].Committer != "tester <tester@example.test>" || len(commits[0].Paths) != 2 {
		t.Fatalf("commits = %+v, %v", commits, err)
	}
	if ident, err := h.committer(t.Context(), repo); err != nil || ident != "tester <tester@example.test>" {
		t.Fatalf("committer = %q, %v", ident, err)
	}
	if _, err := h.commits(t.Context(), repo, strings.Repeat("0", 40), "HEAD", 0); err == nil {
		t.Fatal("a HEAD git does not hold answered commits")
	}
	outside := hook.Repository{Root: t.TempDir()}
	if _, err := h.commits(t.Context(), outside, "", "HEAD", 0); err == nil {
		t.Fatal("a directory outside git answered commits")
	}
	if _, err := h.committer(t.Context(), outside); err == nil {
		t.Fatal("a directory outside git answered a committer")
	}
}

// TestHookPostBoundsTheNoticeOfAPullGitCannotClassify drives through the
// real binding a fast-forward pull of a teammate's commit of thirty files
// holding a dash in a clone whose git var cannot name a committer, as in a
// container whose host name has no domain, which a pull never needs: git
// cannot tell which commits the command created, and the notice names the
// files the move brought in one sentence, twenty by name and the rest
// counted, for the agent and for the person, never as feedback. The test is
// serial because the isolated git configuration lives in the environment
// the git processes inherit.
func TestHookPostBoundsTheNoticeOfAPullGitCannotClassify(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	e := enrolledHook(t)
	files, names := map[string]string{}, make([]string, 0, 30)
	for i := range 30 {
		name := fmt.Sprintf("notes/f%02d.md", i)
		files[name], names = "one \u2014 two\n", append(names, name)
	}
	pushedByMate(t, e.root, files)
	gitRun(t, e.root, "config", "--unset", "user.email")
	gitRun(t, e.root, "config", "--unset", "user.name")
	gitRun(t, e.root, "config", "user.useConfigOnly", "true")
	said := aroundCommand(t, e, "a pull git cannot classify", func(root string) { gitRun(t, root, "pull", "-q", "--ff-only", "origin", "main") })
	if fedBack(said) {
		t.Fatalf("a pull git cannot classify fed back: %q", said)
	}
	agentText, humanText := noticeOf(t, hook.Reply{Stdout: []byte(said)})
	listed := strings.Join(names[:20], ", ")
	if strings.Count(agentText, "golaw did not measure") != 1 ||
		agentText != "golaw did not measure "+listed+" and 10 more: git could not tell which commits this command created; the pipeline judges them." ||
		humanText != "golaw n\u00e3o mediu "+listed+" e mais 10, que o pipeline julga." {
		t.Fatalf("agent %q, human %q", agentText, humanText)
	}
}
