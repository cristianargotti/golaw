package main

import (
	"strings"
	"testing"
)

// limitCases pin what stays a limit of the post event after a move of HEAD:
// a commit the command makes under an identity of its own, through the
// environment or git -c, reads as a commit it did not create, so its file
// is named in the notice; a commit of the same identity made before the pre
// event within the second it started reads as the command's, so a move
// over it judges its file; and a file the command's own commit changed is
// judged whole, a teammate's line a merge brought into it included.
func limitCases(t *testing.T) []moveCase {
	t.Helper()
	dash := "one \u2014 two\n"
	bot := []string{"GIT_COMMITTER_NAME=bot", "GIT_COMMITTER_EMAIL=bot@example.invalid"}
	sameSecond := func(root string) {
		sideBranch(t, root, at(now().Format("2006-01-02T15:04:05Z")), map[string]string{"mine.md": dash})
	}
	whole := func(root string) {
		commitFiles(t, root, map[string]string{"shared.md": "1\n2\n3\n4\n5\n6\n7\n"})
		sideBranch(t, root, mate(earlier), map[string]string{"shared.md": "1\n2\n3\n4\n5\n6\ntheirs \u2014 7\n"})
	}
	return []moveCase{
		{"a commit under a committer of the environment of the command", func(string) {}, func(root string) {
			commitAs(t, root, bot, map[string]string{"mine.md": dash})
		}, "", "mine.md"},
		{"a commit under a committer of git -c", func(string) {}, func(root string) {
			writeFiles(t, root, map[string]string{"mine.md": dash})
			gitRun(t, root, "add", "mine.md")
			gitRun(t, root, "-c", "user.name=bot", "-c", "user.email=bot@example.invalid", "commit", "-q", "-m", "docs: mine")
		}, "", "mine.md"},
		{"a commit of the same identity in the second the pre event started", sameSecond, func(root string) {
			gitRun(t, root, "merge", "-q", "--ff-only", "side")
		}, "mine.md:1", ""},
		{"a teammate's line merged into a file the command committed", whole, func(root string) {
			writeFiles(t, root, map[string]string{"shared.md": "1\nmine 2\n3\n4\n5\n6\n7\n"})
			gitRun(t, root, "commit", "-q", "-a", "-m", "docs: mine")
			gitRun(t, root, "merge", "-q", "--no-commit", "--no-ff", "side")
		}, "shared.md:7", ""},
	}
}

// TestHookPostKeepsTheStatedLimitsOfAMove drives the cases of limitCases
// through the real binding, so a change of any of these limits is a
// decision and never an accident.
func TestHookPostKeepsTheStatedLimitsOfAMove(t *testing.T) {
	t.Parallel()
	for _, tc := range limitCases(t) {
		e := enrolledHook(t)
		tc.before(e.root)
		said := aroundCommand(t, e, tc.name, tc.during)
		judged := tc.judged != "" && fedBack(said) && strings.Contains(said, "("+tc.judged+")")
		named := tc.named == "" || strings.Contains(said, "brought from commits this command did not create, which ask for no fix: "+tc.named)
		if judged != (tc.judged != "") || tc.judged == "" && fedBack(said) || !named {
			t.Errorf("%s: post said %q", tc.name, said)
		}
	}
}
