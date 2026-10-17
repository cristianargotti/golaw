package main

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
)

// enrolledUnborn is the real hook binding over a repository without a
// commit, whose rendered policy is enrolled and left untracked, as init
// --apply leaves a new repository.
func enrolledUnborn(t *testing.T) enrolled {
	t.Helper()
	b, home := testBinding(t)
	root := repository(t, false)
	data := renderedPolicy(t)
	p, err := b.compile(data, now())
	if err != nil {
		t.Fatal(err)
	}
	if err = b.enroll(root, data, p, now()); err != nil {
		t.Fatal(err)
	}
	deps, err := b.hookDependencies(agentUnderTest(), now())
	if err != nil {
		t.Fatal(err)
	}
	return enrolled{deps: deps, root: root, home: home}
}

// aroundCommand runs the shell pre event, the command and its post event
// through the real binding and answers what the post event said, after
// checking that the pre event allowed the command.
func aroundCommand(t *testing.T, e enrolled, name string, during func(root string)) string {
	t.Helper()
	agent := agentUnderTest()
	pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, "true")), agent.Adapter, e.deps)
	during(e.root)
	post := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PostToolUse", e.root, "true")), agent.Adapter, e.deps)
	if pre.ExitCode != 0 || len(pre.Stdout) != 0 || post.ExitCode != 0 {
		t.Fatalf("%s: pre %+v, post %+v", name, pre, post)
	}
	return string(post.Stdout)
}

// fedBack reports whether a post answer feeds the dash rule back.
func fedBack(said string) bool {
	return strings.Contains(said, `"decision":"block"`) && strings.Contains(said, "text.dashes")
}

// TestHookPostJudgesTheFirstCommitOfABranch holds the cases of a command
// that makes the first commit of its branch, through the real binding: the
// tree HEAD held before it is the empty tree, so a dash the command writes
// and commits in the same line is judged, while what the branch already
// held, committed unchanged, a dash a developer left untracked among it, is
// not.
func TestHookPostJudgesTheFirstCommitOfABranch(t *testing.T) {
	t.Parallel()
	dash := "one \u2014 two\n"
	first := func(root string) {
		gitRun(t, root, "add", "-A")
		gitRun(t, root, "commit", "-q", "-m", "build: start")
	}
	for _, tc := range []statusCase{
		{"a dash written and committed as the first commit", nil, func(root string) {
			writeFiles(t, root, map[string]string{"fresh.md": dash})
			first(root)
		}, true},
		{"an untracked dash of a developer committed as the first commit", func(root string) {
			writeFiles(t, root, map[string]string{"draft.md": dash})
		}, first, false},
		{"the first commit of what the branch held", nil, first, false},
	} {
		e := enrolledUnborn(t)
		if tc.before != nil {
			tc.before(e.root)
		}
		said := aroundCommand(t, e, tc.name, tc.during)
		if fedBack(said) != tc.fed || tc.fed && !strings.Contains(said, "fresh.md") || strings.Contains(said, "draft.md") {
			t.Errorf("%s: post said %q", tc.name, said)
		}
	}
}

// TestHookPostComparesAnOrphanCheckoutWithTheCommitItLeft reads the HEAD an
// orphan checkout leaves unborn as the empty tree: every file git status
// then names was staged by the checkout with the bytes the commit it left
// held there, so a dash that commit already held is neither fed back nor
// named.
func TestHookPostComparesAnOrphanCheckoutWithTheCommitItLeft(t *testing.T) {
	t.Parallel()
	e := enrolledHook(t)
	commitFiles(t, e.root, map[string]string{"legacy.md": "one \u2014 two\n"})
	said := aroundCommand(t, e, "an orphan checkout", func(root string) { gitRun(t, root, "checkout", "-q", "--orphan", "pages") })
	if said != "" {
		t.Fatalf("an orphan checkout: post said %q", said)
	}
}

// TestHookPostNamesAMoveGitCannotCompare answers a command whose HEAD git
// can no longer compare, an amend whose replaced commit the command pruned,
// with a notice naming the changes of the command and leaving them to the
// pipeline, never with an allow that says nothing and never with a block.
func TestHookPostNamesAMoveGitCannotCompare(t *testing.T) {
	t.Parallel()
	e := enrolledHook(t)
	said := aroundCommand(t, e, "an amend that prunes the commit it replaced", func(root string) {
		writeFiles(t, root, map[string]string{"fresh.md": "one \u2014 two\n"})
		gitRun(t, root, "add", "fresh.md")
		gitRun(t, root, "commit", "-q", "--amend", "--no-edit")
		gitRun(t, root, "reflog", "expire", "--expire=now", "--all")
		gitRun(t, root, "gc", "-q", "--prune=now")
	})
	if strings.Contains(said, `"decision":"block"`) || !strings.Contains(said, "golaw did not measure the changes of this command") ||
		!strings.Contains(said, "the pipeline judges") {
		t.Fatalf("a move git cannot compare: post said %q", said)
	}
}
