package main

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
)

// movedCases are commands that move HEAD over what they write: a dash a
// command writes and commits in one line, a new dash file it adds and
// commits, a dash it writes and amends into the last commit and a dash
// file a cherry-pick brings are judged, against the blob the HEAD of the
// pre event held, while a commit of dash bytes a developer left dirty or
// untracked, a soft reset and a commit of a clean file are not.
func movedCases(t *testing.T, dash string) []statusCase {
	t.Helper()
	commit := func(root, message string) { gitRun(t, root, "commit", "-q", "-am", message) }
	write := func(root, name, content string) { writeFiles(t, root, map[string]string{name: content}) }
	return []statusCase{
		{"a dash written and committed in one command", nil, func(root string) {
			write(root, "clean.md", dash)
			commit(root, "test: the dash")
		}, true},
		{"a new dash file added and committed", nil, func(root string) {
			write(root, "fresh.md", dash)
			gitRun(t, root, "add", "fresh.md")
			commit(root, "test: the new file")
		}, true},
		{"a dash amended into the last commit", nil, func(root string) {
			write(root, "clean.md", dash)
			gitRun(t, root, "commit", "-q", "-a", "--amend", "--no-edit")
		}, true},
		{"a dash file a cherry-pick brings", func(root string) {
			gitRun(t, root, "checkout", "-q", "-b", "side")
			commitFiles(t, root, map[string]string{"picked.md": dash})
			gitRun(t, root, "checkout", "-q", "-")
		}, func(root string) { gitRun(t, root, "cherry-pick", "side") }, true},
		{"dirty dash bytes of a developer committed", func(root string) { write(root, "clean.md", dash) }, func(root string) {
			commit(root, "test: the developer's dash")
		}, false},
		{"an untracked dash file of a developer committed", func(root string) { write(root, "draft.md", dash) }, func(root string) {
			gitRun(t, root, "add", "-A")
			commit(root, "test: the developer's draft")
		}, false},
		{"a clean file written and committed", nil, func(root string) {
			write(root, "clean.md", "still clean\n")
			commit(root, "test: a clean change")
		}, false},
	}
}

// TestHookPostJudgesWhatACommandCommitted holds the cases of a command
// that moves HEAD through the real binding: git status reports the files
// it wrote and committed clean at both ends, so the paths between the HEAD
// of the pre event and the HEAD after the command are asked of git once and
// each is judged against the blob the earlier HEAD held there.
func TestHookPostJudgesWhatACommandCommitted(t *testing.T) {
	t.Parallel()
	dash := "one \u2014 two\n"
	agent := agentUnderTest()
	for _, tc := range movedCases(t, dash) {
		e := enrolledHook(t)
		commitFiles(t, e.root, map[string]string{"legacy.md": dash, "clean.md": "clean\n"})
		if tc.before != nil {
			tc.before(e.root)
		}
		pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, "true")), agent.Adapter, e.deps)
		tc.during(e.root)
		post := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PostToolUse", e.root, "true")), agent.Adapter, e.deps)
		said := string(post.Stdout)
		fed := strings.Contains(said, `"decision":"block"`) && strings.Contains(said, "text.dashes")
		if pre.ExitCode != 0 || len(pre.Stdout) != 0 || post.ExitCode != 0 || fed != tc.fed || fed && strings.Contains(said, "legacy.md") {
			t.Errorf("%s: pre %+v, post exit %d, stdout %q, stderr %q", tc.name, pre, post.ExitCode, said, post.Stderr)
		}
	}
}
