package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
)

// aroundLine is aroundCommand with the text of the command in both shell
// events, the text the pre event reads the strategy options of a merge in.
func aroundLine(t *testing.T, e enrolled, line string, during func(root string)) string {
	t.Helper()
	agent := agentUnderTest()
	pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, line)), agent.Adapter, e.deps)
	during(e.root)
	post := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PostToolUse", e.root, line)), agent.Adapter, e.deps)
	if pre.ExitCode != 0 || len(pre.Stdout) != 0 || post.ExitCode != 0 {
		t.Fatalf("%s: pre %+v, post %+v", line, pre, post)
	}
	return string(post.Stdout)
}

// mixedTheirs commits shared.md, then a teammate's commit on side that
// puts first in its first line and a local commit that changes its first
// and last lines, so a merge without an option stops on the conflict of the
// first line and -X theirs takes the teammate's line and keeps the local
// last one.
func mixedTheirs(t *testing.T, root, first string) {
	t.Helper()
	commitFiles(t, root, map[string]string{"shared.md": "1\n2\n3\n4\n5\n6\n7\n"})
	sideBranch(t, root, mate(earlier), map[string]string{"shared.md": first + "\n2\n3\n4\n5\n6\n7\n"})
	commitFiles(t, root, map[string]string{"shared.md": "ours 1\n2\n3\n4\n5\n6\nours 7\n"})
}

// mixedOurs commits shared.md, then a teammate's commit on side that
// changes its first line and puts a dash in its fourth, and a local commit
// that changes its first line, so -X ours keeps the local first line and
// merges the teammate's fourth line by itself.
func mixedOurs(t *testing.T, root string) {
	t.Helper()
	commitFiles(t, root, map[string]string{"shared.md": "1\n2\n3\n4\n5\n6\n7\n"})
	sideBranch(t, root, mate(earlier), map[string]string{"shared.md": "theirs 1\n2\n3\ntheirs \u2014 4\n5\n6\n7\n"})
	commitFiles(t, root, map[string]string{"shared.md": "ours 1\n2\n3\n4\n5\n6\n7\n"})
}

// lineCase is a command whose text gives a merge a strategy option: what
// the repository holds before its pre event, the text, what the command
// does, the file a finding names when the post event feeds a dash back and
// what its notice says.
type lineCase struct {
	name, line     string
	before, during func(root string)
	judged         string
	said           []string
}

// strategyCases are the commands that merge a teammate's dash with -X ours
// or -X theirs, left in progress or committed, in the spellings git takes,
// and beside a fallback of the other option that did not run, which name
// what the option took in the notice, and a merge without an option that
// stops on a conflict beside an -X theirs merge that failed after it,
// whose conflict waits; the red case, an edit the command stages after
// such a merge; and the stated limit, an option the text of the command
// does not spell, as a script that runs the merge.
func strategyCases(t *testing.T) []lineCase {
	t.Helper()
	merged := "the merge in progress brought from commits this command did not create, which ask for no fix: shared.md"
	moved := "the move of HEAD brought from commits this command did not create, which ask for no fix: shared.md"
	unmerged := "golaw did not measure shared.md: git left it unmerged, and golaw judges the resolution the agent writes; the pipeline judges it."
	stops := func(root string) { gitStops(t, root, "merge", "-q", "side") }
	conflict := func(root string) { conflicting(t, root, false) }
	merge := func(args ...string) func(string) {
		return func(root string) { gitRun(t, root, append([]string{"merge", "-q"}, args...)...) }
	}
	theirs := func(first string) func(string) { return func(root string) { mixedTheirs(t, root, first) } }
	ours := func(root string) { mixedOurs(t, root) }
	edited := func(root string) {
		merge("--no-commit", "-X", "theirs", "side")(root)
		appendFile(t, root, "shared.md", "mine \u2014 added\n")
		gitRun(t, root, "add", "shared.md")
	}
	pull := func(root string) {
		gitRun(t, root, "pull", "-q", "--no-rebase", "--no-edit", "-X", "ours", ".", "side")
	}
	dash, clean := "theirs \u2014 1", "theirs 1"
	return []lineCase{
		{"a merge -X theirs left in progress", "git merge -q --no-commit -X theirs side", theirs(dash), merge("--no-commit", "-X", "theirs", "side"), "", []string{merged}},
		{"a merge -Xtheirs committed", "git merge -q --no-edit -Xtheirs side", theirs(dash), merge("--no-edit", "-Xtheirs", "side"), "", []string{moved}},
		{"a merge --strategy-option=ours left in progress", "git merge -q --no-commit --strategy-option=ours side", ours, merge("--no-commit", "--strategy-option=ours", "side"), "", []string{merged}},
		{"a pull -X ours committed", "git pull -q --no-rebase --no-edit -X ours . side", ours, pull, "", []string{moved}},
		{
			"a merge -X theirs left in progress, its -X ours fallback not run", "git merge -q --no-commit -X theirs side || git merge -q --no-commit -X ours side",
			theirs(dash), merge("--no-commit", "-X", "theirs", "side"), "",
			[]string{merged},
		},
		{
			"a merge -X theirs committed, its -X ours fallback not run", "git merge -q --no-edit -X theirs side || git merge -q --no-edit -X ours side",
			theirs(dash), merge("--no-edit", "-X", "theirs", "side"), "",
			[]string{moved},
		},
		{"a conflict of a merge without an option beside a failed -X theirs", "git merge -q side || git merge -q -X theirs side", conflict, stops, "", []string{unmerged}},
		{"an edit the command stages after a merge -X theirs", "git merge -q --no-commit -X theirs side && printf 'mine' >> shared.md && git add shared.md", theirs(clean), edited, "shared.md:8", nil},
		{"a merge -X theirs a script runs, the stated limit", "sh merge.sh", theirs(dash), merge("--no-commit", "-X", "theirs", "side"), "shared.md:1", nil},
	}
}

// TestHookPostNamesWhatAStrategyOptionTook drives the cases of
// strategyCases through the real binding: a merge -X ours or -X theirs of a
// teammate's dash, left in progress or committed, is named in the notice
// and never fed back, while what the command wrote after it and a merge
// whose option the text of the command does not spell are judged.
func TestHookPostNamesWhatAStrategyOptionTook(t *testing.T) {
	t.Parallel()
	for _, tc := range strategyCases(t) {
		e := enrolledHook(t)
		tc.before(e.root)
		said := aroundLine(t, e, tc.line, tc.during)
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

// TestHookAsksTheOptionMergeOnlyForWhatThePlainMergeLeaves counts the git
// commands of the post event of a merge -X theirs the command left in
// progress: the merge git makes without an option and the merge with it
// are asked once each, as one git merge-tree and one git cat-file
// --batch-check, beside the questions of an ordinary post event and the git
// hash-object of the merged file, whose bytes are not the blob HEAD holds.
// The test is serial for the reason TestHookAsksGitEachFactOnce gives.
func TestHookAsksTheOptionMergeOnlyForWhatThePlainMergeLeaves(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	e := enrolledHook(t)
	mixedTheirs(t, e.root, "theirs \u2014 1")
	agent := agentUnderTest()
	line := "git merge -q --no-commit -X theirs side"
	pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, line)), agent.Adapter, e.deps)
	gitRun(t, e.root, "merge", "-q", "--no-commit", "-X", "theirs", "side")
	if err := os.WriteFile(trace, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	post := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PostToolUse", e.root, line)), agent.Adapter, e.deps)
	commands := gitCommandsOf(t, trace)
	count := func(name string) int {
		return len(slices.DeleteFunc(slices.Clone(commands), func(c string) bool { return !strings.Contains(c, name) }))
	}
	if pre.ExitCode != 0 || post.ExitCode != 0 || fedBack(string(post.Stdout)) || len(commands) > 10 ||
		count("merge-tree") != 2 || count("-X theirs") != 1 || count("--batch-check") != 2 {
		t.Fatalf("pre %s, post %s, the post event ran %d git commands, the ceiling is 10:\n%s", pre.Stdout, post.Stdout, len(commands), strings.Join(commands, "\n"))
	}
}

// TestHookBindingMergesWithTheOptions reads, through the binding of the
// hook, the merge git makes with -X theirs of HEAD and a teammate's branch,
// which holds the teammate's line where the merge without it holds the
// conflict, and the merge commit -X theirs makes, which holds nothing apart
// from that merge, and answers an error for a commit git does not hold and
// for a directory outside git.
func TestHookBindingMergesWithTheOptions(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	h := hookBindingFor(t, b)
	root := repository(t, true)
	mixedTheirs(t, root, "theirs \u2014 1")
	repo := hook.Repository{Root: root}
	head, side := gitOutput(t, root, "rev-parse", "HEAD"), gitOutput(t, root, "rev-parse", "side")
	blobs, err := h.automaticWith(t.Context(), repo, head, side, []string{"theirs"}, []string{"shared.md"})
	plain, plainErr := h.automatic(t.Context(), repo, head, side, []string{"shared.md"})
	if err != nil || plainErr != nil || len(blobs["shared.md"]) != 2 || blobs["shared.md"][0] == plain["shared.md"][0] {
		t.Fatalf("with -X theirs %v, %v; without %v, %v", blobs, err, plain, plainErr)
	}
	gitRun(t, root, "merge", "-q", "--no-edit", "-X", "theirs", "side")
	merge := gitOutput(t, root, "rev-parse", "HEAD")
	if remerged, err := h.remergedWith(t.Context(), repo, []string{merge}, []string{"theirs"}); err != nil || len(remerged[merge]) != 0 {
		t.Fatalf("remerged with -X theirs %v, %v", remerged, err)
	}
	if _, err := h.automaticWith(t.Context(), repo, head, strings.Repeat("0", 40), []string{"theirs"}, nil); err == nil {
		t.Fatal("a commit git does not hold answered a merge")
	}
	outside := hook.Repository{Root: t.TempDir()}
	if _, err := h.automaticWith(t.Context(), outside, head, side, nil, nil); err == nil {
		t.Fatal("a directory outside git answered a merge")
	}
	if _, err := h.remergedWith(t.Context(), repo, []string{strings.Repeat("0", 40)}, nil); err == nil {
		t.Fatal("a commit git does not hold answered a remerge")
	}
	if _, err := h.remergedWith(t.Context(), outside, []string{merge}, nil); err == nil {
		t.Fatal("a directory outside git answered a remerge")
	}
}
