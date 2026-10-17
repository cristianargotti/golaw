package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
)

// mergeCommitCase is a command that commits a merge: what the repository
// holds before its pre event, what it does, the file a finding names when
// the post event feeds the dash back, and the file the notice names as
// brought from commits the command did not create, none when it names none.
type mergeCommitCase struct {
	name           string
	before, during func(root string)
	judged, named  string
}

// mergeCommitCases are commands that commit a merge of a teammate's branch:
// the red cases, a resolution holding a dash the command writes for a
// conflict an earlier command left, a file it writes into a merge an earlier
// command left in progress, and a merge, a new file and the commit in one
// command; and the near misses, a conflict resolved with the teammate's
// side whole, a clean resolution and an automatic merge of a file both
// sides changed, which judge nothing the command did not write.
func mergeCommitCases(t *testing.T) []mergeCommitCase {
	t.Helper()
	dash := "one \u2014 two\n"
	stopped := func(root string) {
		conflicting(t, root, false)
		gitStops(t, root, "merge", "-q", "side")
	}
	inProgress := func(root string) {
		sideBranch(t, root, mate(earlier), map[string]string{"theirs.md": dash})
		gitRun(t, root, "merge", "-q", "--no-commit", "--no-ff", "side")
	}
	commit := func(write func(root string), add string) func(string) {
		return func(root string) {
			write(root)
			gitRun(t, root, "add", add)
			gitRun(t, root, "commit", "-q", "--no-edit")
		}
	}
	file := func(name, content string) func(string) {
		return func(root string) { writeFiles(t, root, map[string]string{name: content}) }
	}
	merge := func(root string) { gitRun(t, root, "merge", "-q", "--no-commit", "--no-ff", "side") }
	return []mergeCommitCase{
		{"a resolution holding a dash the command commits", stopped, commit(file("shared.md", "mine \u2014 resolved\n"), "shared.md"), "shared.md:1", ""},
		{"a file the command commits into a merge in progress", inProgress, commit(file("mine.md", dash), "-A"), "mine.md:1", ""},
		{"a merge, a new file and the commit in one command", func(root string) {
			sideBranch(t, root, mate(earlier), map[string]string{"theirs.md": dash})
		}, commit(func(root string) { merge(root); file("mine.md", dash)(root) }, "-A"), "mine.md:1", "theirs.md"},
		{"a conflict resolved with the teammate's side whole", stopped, commit(func(root string) {
			gitRun(t, root, "checkout", "-q", "--theirs", "shared.md")
		}, "shared.md"), "", "shared.md"},
		{"a clean resolution", stopped, commit(file("shared.md", "clean resolution\n"), "shared.md"), "", ""},
		{"an automatic merge commit of a file both sides changed", func(root string) { bothSides(t, root) }, func(root string) {
			gitRun(t, root, "merge", "-q", "--no-ff", "--no-edit", "side")
		}, "", "shared.md"},
	}
}

// TestHookPostJudgesWhatAMergeCommitOfTheCommandHolds drives the cases of
// mergeCommitCases through the real binding: of a merge commit the command
// made, the files it holds apart both from every parent and from the merge
// git makes of them by itself are judged, and every other file the merge
// brought is named in the notice that asks for no fix.
func TestHookPostJudgesWhatAMergeCommitOfTheCommandHolds(t *testing.T) {
	t.Parallel()
	brought := "brought from commits this command did not create, which ask for no fix: "
	for _, tc := range mergeCommitCases(t) {
		e := enrolledHook(t)
		tc.before(e.root)
		said := aroundCommand(t, e, tc.name, tc.during)
		judged := tc.judged != "" && fedBack(said) && strings.Contains(said, "("+tc.judged+")")
		named := tc.named == "" && !strings.Contains(said, "brought") || tc.named != "" && strings.Contains(said, brought+tc.named+";")
		if judged != (tc.judged != "") || tc.judged == "" && fedBack(said) || !named {
			t.Errorf("%s: post said %q", tc.name, said)
		}
	}
}

// TestHookAsksGitTheRemergeOnlyForAMergeOfTheCommand counts the git
// commands of the post event of a merge commit the command made: the
// remerge is asked of git once, as one git log --remerge-diff, beside the
// questions of a move of HEAD and the listing the judged resolution needs,
// when the merge holds a path the move brought apart from every parent,
// here the command's resolution of a conflict, and never when the merge
// took every file from one side whole. The test is serial for the reason
// TestHookAsksGitEachFactOnce gives.
func TestHookAsksGitTheRemergeOnlyForAMergeOfTheCommand(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	agent := agentUnderTest()
	for _, tc := range []struct {
		name             string
		resolved         bool
		ceiling, remerge int
	}{{"a resolution the command commits", true, 10, 1}, {"a merge of a teammate's file", false, 8, 0}} {
		e := enrolledHook(t)
		if tc.resolved {
			conflicting(t, e.root, false)
			gitStops(t, e.root, "merge", "-q", "side")
		} else {
			sideBranch(t, e.root, mate(earlier), map[string]string{"theirs.md": "clean\n"})
		}
		pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, "true")), agent.Adapter, e.deps)
		if tc.resolved {
			writeFiles(t, e.root, map[string]string{"shared.md": "resolved\n"})
			gitRun(t, e.root, "commit", "-q", "-a", "--no-edit")
		} else {
			gitRun(t, e.root, "merge", "-q", "--no-ff", "--no-edit", "side")
		}
		if err := os.WriteFile(trace, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		post := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PostToolUse", e.root, "true")), agent.Adapter, e.deps)
		commands := gitCommandsOf(t, trace)
		remerge := len(slices.DeleteFunc(slices.Clone(commands), func(c string) bool { return !strings.Contains(c, "--remerge-diff") }))
		if pre.ExitCode != 0 || post.ExitCode != 0 || fedBack(string(post.Stdout)) || len(commands) > tc.ceiling || remerge != tc.remerge {
			t.Fatalf("%s: pre %+v, post %+v, the post event ran %d git commands, the ceiling is %d:\n%s",
				tc.name, pre, post, len(commands), tc.ceiling, strings.Join(commands, "\n"))
		}
	}
}

// TestHookBindingRemergesAsGitMergesByItself reads, through the binding of
// the hook, a merge commit the way git log names it, as a merge with the
// paths it holds apart from every parent, and the paths it holds apart from
// the merge git makes of its parents by itself, and answers an error for a
// commit git does not hold and for a directory outside git.
func TestHookBindingRemergesAsGitMergesByItself(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	h := hookBindingFor(t, b)
	root := repository(t, true)
	conflicting(t, root, false)
	start := gitOutput(t, root, "rev-parse", "HEAD")
	gitStops(t, root, "merge", "-q", "side")
	writeFiles(t, root, map[string]string{"shared.md": "resolved\n"})
	gitRun(t, root, "commit", "-q", "-a", "--no-edit")
	merge, repo := gitOutput(t, root, "rev-parse", "HEAD"), hook.Repository{Root: root}
	commits, err := h.commits(t.Context(), repo, start, merge, 0)
	if err != nil || len(commits) != 2 || !commits[0].Merge || commits[0].Name != merge || !slices.Equal(commits[0].Paths, []string{"shared.md"}) {
		t.Fatalf("commits = %+v, %v", commits, err)
	}
	if remerged, err := h.remerged(t.Context(), repo, []string{merge}); err != nil || !slices.Equal(remerged[merge], []string{"shared.md"}) {
		t.Fatalf("remerged = %v, %v", remerged, err)
	}
	if _, err := h.remerged(t.Context(), repo, []string{strings.Repeat("0", 40)}); err == nil {
		t.Fatal("a commit git does not hold answered a remerge")
	}
	if _, err := h.remerged(t.Context(), hook.Repository{Root: t.TempDir()}, []string{merge}); err == nil {
		t.Fatal("a directory outside git answered a remerge")
	}
}
