package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
)

// elsewhere commits shared.md, then a teammate's commit on side that
// changes its last line, which the local branch leaves alone, so a merge of
// side changes shared.md as git merges it by itself.
func elsewhere(t *testing.T, root string) {
	t.Helper()
	commitFiles(t, root, map[string]string{"shared.md": "1\n2\n3\n4\n5\n6\n7\n"})
	sideBranch(t, root, mate(earlier), map[string]string{"shared.md": "1\n2\n3\n4\n5\n6\ntheirs 7\n"})
}

// ownSide commits a dash of the command in shared.md, then merges side,
// which stops on the conflict of that file, and resolves it with the side
// of HEAD whole without git add.
func ownSide(t *testing.T, root string) {
	t.Helper()
	writeFiles(t, root, map[string]string{"shared.md": "mine \u2014 line\n"})
	gitRun(t, root, "commit", "-q", "-a", "-m", "docs: mine")
	gitStops(t, root, "merge", "-q", "side")
	gitRun(t, root, "checkout", "-q", "--ours", "shared.md")
}

// ownMergeCases are the commands that commit and then start a merge they
// leave in progress: the red cases, a dash the command commits in shared.md
// before it merges a teammate's change of the same file, and before a
// conflict it resolves with its own side without git add; and the near
// misses, a teammate's commit checked out before such a merge and a commit
// of the command that leaves shared.md alone, where the merge brought
// shared.md and the notice names it.
func ownMergeCases(t *testing.T) []mergeCase {
	t.Helper()
	merged := "the merge in progress brought from commits this command did not create, which ask for no fix: shared.md"
	merge := func(root string) { gitRun(t, root, "merge", "-q", "--no-commit", "--no-ff", "side") }
	commit := func(files map[string]string) func(string) {
		return func(root string) {
			writeFiles(t, root, files)
			gitRun(t, root, "add", "-A")
			gitRun(t, root, "commit", "-q", "-m", "docs: mine")
			merge(root)
		}
	}
	other := func(root string) {
		elsewhere(t, root)
		gitRun(t, root, "checkout", "-q", "-b", "other")
		commitAs(t, root, mate(earlier), map[string]string{"shared.md": "1\nmate \u2014 2\n3\n4\n5\n6\n7\n"})
		gitRun(t, root, "checkout", "-q", "-")
	}
	return []mergeCase{
		{
			"a dash the command commits, then a merge it leaves in progress", func(root string) { elsewhere(t, root) },
			commit(map[string]string{"shared.md": "1\nmine \u2014 2\n3\n4\n5\n6\n7\n"}), "shared.md:2", nil,
		},
		{
			"a dash the command commits, then a conflict it resolves with its own side", func(root string) { conflicting(t, root, false) },
			func(root string) { ownSide(t, root) }, "shared.md:1", nil,
		},
		{"a teammate's commit checked out, then a merge left in progress", other, func(root string) {
			gitRun(t, root, "checkout", "-q", "other")
			merge(root)
		}, "", []string{merged}},
		{
			"a commit of the command elsewhere, then a merge left in progress", func(root string) { elsewhere(t, root) },
			commit(map[string]string{"mine.md": "mine\n"}), "",
			[]string{merged},
		},
	}
}

// TestHookPostJudgesTheCommitOfTheCommandBeforeAMerge drives the cases of
// ownMergeCases through the real binding: a file a commit of the command
// changed before it started a merge is judged, while a file only the merge
// changed is named in the notice and never fed back.
func TestHookPostJudgesTheCommitOfTheCommandBeforeAMerge(t *testing.T) {
	t.Parallel()
	for _, tc := range ownMergeCases(t) {
		e := enrolledHook(t)
		tc.before(e.root)
		said := aroundCommand(t, e, tc.name, tc.during)
		judged := tc.judged != "" && fedBack(said) && strings.Contains(said, "("+tc.judged+")")
		named := true
		for _, part := range tc.said {
			named = named && strings.Contains(said, part)
		}
		if judged != (tc.judged != "") || tc.judged == "" && fedBack(said) || !named {
			t.Errorf("%s: post said %q", tc.name, said)
		}
	}
}

// TestHookAsksTheCommitsOnceForACommitThenAMerge counts the git commands of
// the post event of a command that commits its own change and then starts
// a merge it leaves in progress: the move, the commits and the committer
// are each asked once, beside the merge git makes by itself, the git
// hash-object of the merged file, the listing the judged file needs and the
// questions of an ordinary post event. The test is serial for the reason
// TestHookAsksGitEachFactOnce gives.
func TestHookAsksTheCommitsOnceForACommitThenAMerge(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	e := enrolledHook(t)
	elsewhere(t, e.root)
	agent := agentUnderTest()
	pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, "true")), agent.Adapter, e.deps)
	commitFiles(t, e.root, map[string]string{"shared.md": "1\nmine \u2014 2\n3\n4\n5\n6\n7\n"})
	gitRun(t, e.root, "merge", "-q", "--no-commit", "--no-ff", "side")
	if err := os.WriteFile(trace, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	post := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PostToolUse", e.root, "true")), agent.Adapter, e.deps)
	commands := gitCommandsOf(t, trace)
	count := func(name string) int {
		return len(slices.DeleteFunc(slices.Clone(commands), func(c string) bool { return !strings.Contains(c, name) }))
	}
	if pre.ExitCode != 0 || post.ExitCode != 0 || !fedBack(string(post.Stdout)) || len(commands) > 12 ||
		count("diff-tree") != 1 || count(" var GIT_COMMITTER_IDENT") != 1 || count(" log ") != 1 || count("merge-tree") != 1 {
		t.Fatalf("pre %s, post %s, the post event ran %d git commands, the ceiling is 12:\n%s", pre.Stdout, post.Stdout, len(commands), strings.Join(commands, "\n"))
	}
}
