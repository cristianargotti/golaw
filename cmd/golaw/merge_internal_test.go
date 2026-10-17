package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
)

// gitStops runs a git command that stops on a conflict, as a pull or a
// rebase does, and fails the test when it does not.
func gitStops(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("git %v did not stop on its conflict:\n%s", args, out)
	}
}

// appendFile appends a line to a file of a test under the root.
func appendFile(t *testing.T, root, name, line string) {
	t.Helper()
	file, err := os.OpenFile(filepath.Join(root, filepath.FromSlash(name)), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString(line); err != nil {
		t.Fatal(err)
	}
}

// mergeCase is a command that merges a teammate's commit: what the
// repository holds before its pre event, what it does, the file a finding
// names when the post event feeds the dash back, and what its notice says.
type mergeCase struct {
	name           string
	before, during func(root string)
	judged         string
	said           []string
}

// conflicting commits shared.md, then a teammate's commit that changes it
// with a dash and adds theirs.md holding one, on the branch side or, when
// remote is true, on the main branch of a bare remote named origin, and a
// local commit that changes shared.md too, so merging the two stops on a
// conflict.
func conflicting(t *testing.T, root string, remote bool) {
	t.Helper()
	commitFiles(t, root, map[string]string{"shared.md": "start\n"})
	theirs := map[string]string{"shared.md": "mate \u2014 line\n", "theirs.md": "one \u2014 two\n"}
	if remote {
		pushedByMate(t, root, theirs)
	} else {
		sideBranch(t, root, mate(earlier), theirs)
	}
	commitFiles(t, root, map[string]string{"shared.md": "local line\n"})
}

// bothSides commits shared.md, then a teammate's commit on side that puts a
// dash in its last line and a local commit that changes its first, so git
// merges the two sides by itself.
func bothSides(t *testing.T, root string) {
	t.Helper()
	commitFiles(t, root, map[string]string{"shared.md": "a\nb\nc\n"})
	sideBranch(t, root, mate(earlier), map[string]string{"shared.md": "a\nb\none \u2014 two\n"})
	commitFiles(t, root, map[string]string{"shared.md": "local\nb\nc\n"})
}

// mergeCases are the commands that merge a teammate's commit holding a
// dash: a pull that stops on a conflict, a merge --no-commit, an automatic
// merge of a file both sides changed and a rebase that stops on a
// conflict, which only name what git wrote, and the red cases, a file the
// command writes after the merge, its edit of a merged file and a file it
// writes while a developer's merge is in progress; last, the stated limit,
// a merged file the command edits and stages again, named as merged.
func mergeCases(t *testing.T) []mergeCase {
	t.Helper()
	dash := "one \u2014 two\n"
	unmerged := "golaw did not measure shared.md: git left it unmerged, and golaw judges the resolution the agent writes; the pipeline judges it."
	merged := "the merge in progress brought from commits this command did not create, which ask for no fix: "
	moved := "the move of HEAD brought from commits this command did not create, which ask for no fix: theirs.md"
	theirs := func(content string) func(string) {
		return func(root string) { sideBranch(t, root, mate(earlier), map[string]string{"theirs.md": content}) }
	}
	merge := func(root string) { gitRun(t, root, "merge", "-q", "--no-commit", "--no-ff", "side") }
	then := func(first, second func(string)) func(string) { return func(root string) { first(root); second(root) } }
	mine := func(root string) { writeFiles(t, root, map[string]string{"mine.md": dash}) }
	fromRemote, fromSide := func(root string) { conflicting(t, root, true) }, func(root string) { conflicting(t, root, false) }
	pull := func(root string) { gitStops(t, root, "pull", "-q", "--no-rebase", "origin", "main") }
	rebase := func(root string) { gitStops(t, root, "rebase", "side") }
	restaged := func(root string) {
		appendFile(t, root, "theirs.md", dash)
		gitRun(t, root, "add", "theirs.md")
	}
	return []mergeCase{
		{"a pull that stops on a conflict", fromRemote, pull, "", []string{unmerged, merged + "theirs.md"}},
		{"a merge --no-commit of a teammate's branch", theirs(dash), merge, "", []string{merged + "theirs.md"}},
		{"an automatic merge of a file both sides changed", func(root string) { bothSides(t, root) }, merge, "", []string{merged + "shared.md"}},
		{"a rebase that stops on a conflict", fromSide, rebase, "", []string{unmerged, moved}},
		{"a file the command writes after a merge", theirs(dash), then(merge, mine), "mine.md:1", []string{merged + "theirs.md"}},
		{"an edit of a merged file", theirs("clean\n"), then(merge, func(root string) { appendFile(t, root, "theirs.md", dash) }), "theirs.md:2", nil},
		{"a file written while a developer's merge is in progress", then(theirs(dash), merge), mine, "mine.md:1", nil},
		{"a merged file edited and staged again, the stated limit", theirs("clean\n"), then(merge, restaged), "", []string{merged + "theirs.md"}},
	}
}

// TestHookPostLeavesWhatAMergeBrought drives the cases of mergeCases
// through the real binding: what git wrote for a merge the command started,
// the files it staged from the teammate's commit and a file it left
// unmerged, is named in the notice and never fed back, while a file the
// command wrote, after the merge or during a developer's, is.
func TestHookPostLeavesWhatAMergeBrought(t *testing.T) {
	t.Parallel()
	for _, tc := range mergeCases(t) {
		e := enrolledHook(t)
		tc.before(e.root)
		said := aroundCommand(t, e, tc.name, tc.during)
		judged := tc.judged != "" && fedBack(said) && strings.Contains(said, "("+tc.judged+")")
		named := true
		for _, part := range tc.said {
			named = named && strings.Contains(said, part)
		}
		if judged != (tc.judged != "") || tc.judged == "" && fedBack(said) || !named ||
			tc.judged != "" && tc.said == nil && strings.Contains(said, "brought") {
			t.Errorf("%s: post said %q", tc.name, said)
		}
	}
}

// TestHookAsksGitOnceMoreForAMergeInProgress counts the git commands of the
// post event of a merge --no-commit of a teammate's branch: what the merge
// brought is asked of git once, as one git diff-tree, beside the questions
// of an ordinary post event, and neither git log nor git var runs, because
// HEAD stayed in place. The test is serial for the reason
// TestHookAsksGitEachFactOnce gives.
func TestHookAsksGitOnceMoreForAMergeInProgress(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	e := enrolledHook(t)
	sideBranch(t, e.root, mate(earlier), map[string]string{"theirs.md": "clean\n"})
	agent := agentUnderTest()
	pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, "true")), agent.Adapter, e.deps)
	gitRun(t, e.root, "merge", "-q", "--no-commit", "--no-ff", "side")
	if err := os.WriteFile(trace, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	post := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PostToolUse", e.root, "true")), agent.Adapter, e.deps)
	commands := gitCommandsOf(t, trace)
	count := func(name string) int {
		return len(slices.DeleteFunc(slices.Clone(commands), func(c string) bool { return !strings.Contains(c, name) }))
	}
	if pre.ExitCode != 0 || post.ExitCode != 0 || !strings.Contains(string(post.Stdout), "the merge in progress brought") ||
		len(commands) > 6 || count("diff-tree") != 1 || count(" log ") != 0 || count(" var ") != 0 {
		t.Fatalf("pre %+v, post %+v, the post event ran %d git commands, the ceiling is 6:\n%s", pre, post, len(commands), strings.Join(commands, "\n"))
	}
}
