package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
)

// amendable commits a teammate's notes.md holding a dash, then the
// developer's commit, made before the command, that fixes it and adds
// keep.md, so HEAD holds the fix and its parent the dash.
func amendable(t *testing.T, root string) {
	t.Helper()
	commitAs(t, root, mate(earlier), map[string]string{"notes.md": "start\nold \u2014 text\n"})
	commitAs(t, root, at(earlier), map[string]string{"notes.md": "start\nold text\n", "keep.md": "keep\n"})
}

// amendCases are the commands that rewrite or leave the developer's last
// commit, each with the command text the hook reads: the red cases, an
// amend that checks notes.md out of the parent, one that writes the
// parent's bytes back and one that writes a dash of its own; the near
// misses, an amend that keeps the fix, a reset to the parent and a reset
// followed by a commit on the same parent, where the reset brought
// notes.md and the notice names it; and the stated limit, an amend a script
// runs, which the hook does not read as an amend.
func amendCases(t *testing.T) []lineCase {
	t.Helper()
	amend := func(files map[string]string) func(string) {
		return func(root string) {
			writeFiles(t, root, files)
			gitRun(t, root, "commit", "-q", "-a", "--amend", "--no-edit")
		}
	}
	restore := func(root string) {
		gitRun(t, root, "checkout", "-q", "HEAD~1", "--", "notes.md")
		gitRun(t, root, "commit", "-q", "--amend", "--no-edit")
	}
	reset := func(root string) { gitRun(t, root, "reset", "-q", "--hard", "HEAD~1") }
	resetCommit := func(root string) {
		reset(root)
		gitRun(t, root, "commit", "-q", "--allow-empty", "-m", "docs: again")
	}
	before := func(root string) { amendable(t, root) }
	named := []string{"brought from commits this command did not create, which ask for no fix: notes.md"}
	restoring := "git checkout -q HEAD~1 -- notes.md && git commit -q --amend --no-edit"
	amending := "git commit -q -a --amend --no-edit"
	return []lineCase{
		{"an amend that checks the parent's notes.md out", restoring, before, restore, "notes.md:2", nil},
		{"an amend that writes the parent's bytes back", amending, before, amend(map[string]string{"notes.md": "start\nold \u2014 text\n"}), "notes.md:2", nil},
		{"an amend that writes a dash of its own", amending, before, amend(map[string]string{"notes.md": "start\nnew \u2014 text\n"}), "notes.md:2", nil},
		{"an amend that keeps the fix", amending, before, amend(map[string]string{"keep.md": "kept\n"}), "", nil},
		{"a reset to the parent", "git reset -q --hard HEAD~1", before, reset, "", named},
		{"a reset and a commit on the same parent", "git reset -q --hard HEAD~1 && git commit -q --allow-empty -m 'docs: again'", before, resetCommit, "", named},
		{"an amend a script runs, the stated limit", "sh amend.sh", before, restore, "", named},
	}
}

// TestHookPostJudgesWhatAnAmendRestored drives the cases of amendCases
// through the real binding: every file an amend of the command changed
// against the commit it rewrote is judged, a file restored to the bytes of
// its parent included, while what a reset brought is named in the notice,
// a commit the command makes on the same parent after it included.
func TestHookPostJudgesWhatAnAmendRestored(t *testing.T) {
	t.Parallel()
	for _, tc := range amendCases(t) {
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

// TestHookAsksTheParentsOnceForAnAmend counts the git commands of the post
// event of an amend that restores a file to its parent's bytes: the
// parents of the amended HEAD are asked once, as one git rev-parse, beside
// the questions of a move, and no second git diff-tree runs, because the
// amend is HEAD. The test is serial for the reason
// TestHookAsksGitEachFactOnce gives.
func TestHookAsksTheParentsOnceForAnAmend(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	e := enrolledHook(t)
	amendable(t, e.root)
	agent := agentUnderTest()
	line := "git checkout -q HEAD~1 -- notes.md && git commit -q --amend --no-edit"
	pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, line)), agent.Adapter, e.deps)
	gitRun(t, e.root, "checkout", "-q", "HEAD~1", "--", "notes.md")
	gitRun(t, e.root, "commit", "-q", "--amend", "--no-edit")
	if err := os.WriteFile(trace, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	post := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PostToolUse", e.root, line)), agent.Adapter, e.deps)
	commands := gitCommandsOf(t, trace)
	count := func(name string) int {
		return len(slices.DeleteFunc(slices.Clone(commands), func(c string) bool { return !strings.Contains(c, name) }))
	}
	if pre.ExitCode != 0 || post.ExitCode != 0 || !fedBack(string(post.Stdout)) || len(commands) > 10 ||
		count("^@") != 1 || count("diff-tree") != 1 || count(" log ") != 1 {
		t.Fatalf("pre %s, post %s, the post event ran %d git commands, the ceiling is 10:\n%s", pre.Stdout, post.Stdout, len(commands), strings.Join(commands, "\n"))
	}
}

// TestHookBindingNamesTheParents reads, through the binding of the hook,
// the parents of a commit and of a root commit, the same parents git log
// names beside each commit, and answers an error for a commit git does not
// hold and for a directory outside git.
func TestHookBindingNamesTheParents(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	h := hookBindingFor(t, b)
	root := repository(t, true)
	first := gitOutput(t, root, "rev-parse", "HEAD")
	commitFiles(t, root, map[string]string{"notes.md": "notes\n"})
	second := gitOutput(t, root, "rev-parse", "HEAD")
	repo := hook.Repository{Root: root}
	parents, err := h.parents(t.Context(), repo, second)
	none, rootErr := h.parents(t.Context(), repo, first)
	if err != nil || rootErr != nil || !slices.Equal(parents, []string{first}) || len(none) != 0 {
		t.Fatalf("parents %v, %v; of the root commit %v, %v", parents, err, none, rootErr)
	}
	if commits, err := h.commits(t.Context(), repo, first, second, 0); err != nil || len(commits) != 1 || !slices.Equal(commits[0].Parents, []string{first}) {
		t.Fatalf("commits %+v, %v", commits, err)
	}
	if _, err := h.parents(t.Context(), repo, strings.Repeat("0", 40)); err == nil {
		t.Fatal("a commit git does not hold answered its parents")
	}
	if _, err := h.parents(t.Context(), hook.Repository{Root: t.TempDir()}, second); err == nil {
		t.Fatal("a directory outside git answered parents")
	}
}
