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

// gitOutput runs one git command in a directory and answers what it
// printed, trimmed, failing the test on an error.
func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
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

// localOnly commits, after the fork of a teammate's branch side that adds
// theirs.md holding a dash, notes.md as the local branch alone holds it, so
// the merge of side never touches it; before is true when notes.md was
// committed before the fork too and the local branch alone changed it since.
func localOnly(t *testing.T, root string, before bool) {
	t.Helper()
	if before {
		commitFiles(t, root, map[string]string{"notes.md": "old notes\n"})
	}
	sideBranch(t, root, mate(earlier), map[string]string{"theirs.md": "one \u2014 two\n"})
	commitFiles(t, root, map[string]string{"notes.md": "local notes\n"})
}

// stagedAfterMerge is a command that merges side, leaving the merge in
// progress, appends a line holding a dash to a file and stages it with git
// add of the argument given.
func stagedAfterMerge(t *testing.T, name, add string) func(string) {
	t.Helper()
	return func(root string) {
		gitRun(t, root, "merge", "-q", "--no-commit", "--no-ff", "side")
		appendFile(t, root, name, "one \u2014 two\n")
		gitRun(t, root, "add", add)
	}
}

// resolvedConflict is a command whose merge of side stops on the conflict
// of shared.md, which resolve resolves, and that stages the resolution.
func resolvedConflict(t *testing.T, resolve func(root string)) func(string) {
	t.Helper()
	return func(root string) {
		gitStops(t, root, "merge", "-q", "side")
		resolve(root)
		gitRun(t, root, "add", "shared.md")
	}
}

// mergeCases are the commands that merge a teammate's commit holding a
// dash: a pull that stops on a conflict, a merge --no-commit, an automatic
// merge of a file both sides changed, a conflict whose teammate's side the
// command takes whole and a rebase that stops on a conflict, which only
// name what git wrote; and the red cases, whose bytes differ from the merge
// git makes by itself and from each side: a file the command writes after
// the merge, its edit of a merged file, staged or not, its edit of a file
// only the local side added or changed since the fork, staged by name or
// with git add -A, its own resolution of a conflict and a file it writes
// while a developer's merge is in progress.
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
	local := func(before bool) func(string) { return func(root string) { localOnly(t, root, before) } }
	pull := func(root string) { gitStops(t, root, "pull", "-q", "--no-rebase", "origin", "main") }
	rebase := func(root string) { gitStops(t, root, "rebase", "side") }
	takeTheirs := resolvedConflict(t, func(root string) { gitRun(t, root, "checkout", "-q", "--theirs", "shared.md") })
	ownResolution := resolvedConflict(t, func(root string) { writeFiles(t, root, map[string]string{"shared.md": "mine \u2014 resolved\n"}) })
	return []mergeCase{
		{"a pull that stops on a conflict", fromRemote, pull, "", []string{unmerged, merged + "theirs.md"}},
		{"a merge --no-commit of a teammate's branch", theirs(dash), merge, "", []string{merged + "theirs.md"}},
		{"an automatic merge of a file both sides changed", func(root string) { bothSides(t, root) }, merge, "", []string{merged + "shared.md"}},
		{"a conflict whose teammate's side the command takes whole", fromSide, takeTheirs, "", []string{merged + "shared.md, theirs.md"}},
		{"a rebase that stops on a conflict", fromSide, rebase, "", []string{unmerged, moved}},
		{"a file the command writes after a merge", theirs(dash), then(merge, mine), "mine.md:1", []string{merged + "theirs.md"}},
		{"an edit of a merged file", theirs("clean\n"), then(merge, func(root string) { appendFile(t, root, "theirs.md", dash) }), "theirs.md:2", nil},
		{"a merged file edited and staged again", theirs("clean\n"), stagedAfterMerge(t, "theirs.md", "theirs.md"), "theirs.md:2", nil},
		{"a file only the local side added, edited and staged", local(false), stagedAfterMerge(t, "notes.md", "notes.md"), "notes.md:2", []string{merged + "theirs.md"}},
		{"a file only the local side added, edited and staged with git add -A", local(false), stagedAfterMerge(t, "notes.md", "-A"), "notes.md:2", []string{merged + "theirs.md"}},
		{"a file only the local side changed since the fork, edited and staged", local(true), stagedAfterMerge(t, "notes.md", "notes.md"), "notes.md:2", []string{merged + "theirs.md"}},
		{"a conflict the command resolves with a dash of its own", fromSide, ownResolution, "shared.md:1", []string{merged + "theirs.md"}},
		{"a file written while a developer's merge is in progress", then(theirs(dash), merge), mine, "mine.md:1", nil},
	}
}

// mergeLimits are the stated limits of a merge the command leaves in
// progress: a merge -X theirs that resolves a conflict with the teammate's
// line while keeping a local line elsewhere differs from the merge git
// makes by itself and from each side, so the file is judged as the
// command's resolution; an octopus merge, which git merge-tree cannot make,
// leaves each file it staged to a notice.
func mergeLimits(t *testing.T) []mergeCase {
	t.Helper()
	mixed := func(root string) {
		commitFiles(t, root, map[string]string{"shared.md": "1\n2\n3\n4\n5\n6\n7\n"})
		sideBranch(t, root, mate(earlier), map[string]string{"shared.md": "theirs \u2014 1\n2\n3\ntheirs 4\n5\n6\n7\n"})
		commitFiles(t, root, map[string]string{"shared.md": "ours 1\n2\n3\n4\n5\n6\nours 7\n"})
	}
	branches := func(root string) {
		start := gitOutput(t, root, "rev-parse", "--abbrev-ref", "HEAD")
		for _, name := range []string{"one", "two"} {
			gitRun(t, root, "checkout", "-q", "-b", name, start)
			commitAs(t, root, mate(earlier), map[string]string{name + ".md": "one \u2014 two\n"})
		}
		gitRun(t, root, "checkout", "-q", start)
	}
	octopus := "golaw did not measure one.md: git could not make the merge in progress by itself to compare it; the pipeline judges it."
	return []mergeCase{
		{"a merge -X theirs that mixes both sides, the stated limit", mixed, func(root string) {
			gitRun(t, root, "merge", "-q", "--no-commit", "-X", "theirs", "side")
		}, "shared.md:1", nil},
		{"an octopus merge left in progress, the stated limit", branches, func(root string) {
			gitRun(t, root, "merge", "-q", "--no-commit", "one", "two")
		}, "", []string{octopus}},
	}
}

// TestHookPostLeavesWhatAMergeBrought drives the cases of mergeCases
// through the real binding: what git wrote for a merge the command started,
// the files it staged from the teammate's commit and a file it left
// unmerged, is named in the notice and never fed back, while a file the
// command wrote, after the merge or during a developer's, is.
func TestHookPostLeavesWhatAMergeBrought(t *testing.T) {
	t.Parallel()
	for _, tc := range append(mergeCases(t), mergeLimits(t)...) {
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
// post event of a merge --no-commit of a teammate's branch: the merge git
// makes by itself is asked of git once, as one git merge-tree, and the
// blobs it and the merged commit hold at the staged files once, as one git
// cat-file --batch-check, beside the questions of an ordinary post event,
// and neither git diff-tree, git log nor git var runs, because HEAD stayed
// in place. The test is serial for the reason TestHookAsksGitEachFactOnce
// gives.
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
		len(commands) > 7 || count("merge-tree") != 1 || count("--batch-check") != 1 || count("diff-tree") != 0 || count(" log ") != 0 || count(" var ") != 0 {
		t.Fatalf("pre %+v, post %+v, the post event ran %d git commands, the ceiling is 7:\n%s", pre, post, len(commands), strings.Join(commands, "\n"))
	}
}

// TestHookBindingMergesAsGitMergesByItself reads, through the binding of
// the hook, the blobs of the merge git makes of HEAD and a teammate's
// branch by itself and of that branch, and answers an error for a commit
// git does not hold and for a directory outside git.
func TestHookBindingMergesAsGitMergesByItself(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	h := hookBindingFor(t, b)
	root := repository(t, true)
	sideBranch(t, root, mate(earlier), map[string]string{"theirs.md": "clean\n"})
	repo := hook.Repository{Root: root}
	head, side := gitOutput(t, root, "rev-parse", "HEAD"), gitOutput(t, root, "rev-parse", "side")
	blobs, err := h.automatic(t.Context(), repo, head, side, []string{"theirs.md"})
	if theirs := gitOutput(t, root, "rev-parse", "side:theirs.md"); err != nil || !slices.Equal(blobs["theirs.md"], []string{theirs, theirs}) {
		t.Fatalf("blobs = %v, %v", blobs, err)
	}
	if _, err := h.automatic(t.Context(), repo, head, strings.Repeat("0", 40), []string{"theirs.md"}); err == nil {
		t.Fatal("a commit git does not hold answered a merge")
	}
	if _, err := h.automatic(t.Context(), hook.Repository{Root: t.TempDir()}, head, side, nil); err == nil {
		t.Fatal("a directory outside git answered a merge")
	}
}
