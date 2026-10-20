package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/setup"
)

// receiptOf writes the receipt golaw setup leaves in the state directory of
// b after it armed the hooks directory hooks from the worktree at root.
func receiptOf(t *testing.T, b binding, root, hooks string) {
	t.Helper()
	dirs, err := b.dirs()
	if err != nil {
		t.Fatal(err)
	}
	canonical, identity, err := install.ProjectIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	receipt := setup.Receipt{
		Schema: setup.ReceiptSchema, Identity: identity, Root: canonical, Version: testVersion,
		HooksPath: hooks, ArmedAt: now(),
	}
	if err := setup.WriteReceipt(setup.ReceiptPath(dirs.State, identity), receipt); err != nil {
		t.Fatal(err)
	}
}

// asHook is the binding whose environment holds what git gives a hook it
// runs in the linked worktree at dir: GIT_DIR names the worktree's own git
// directory, so a child git that keeps it ignores its working directory.
func asHook(t *testing.T, b binding, dir string) binding {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", "rev-parse", "--absolute-git-dir")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	gitDir := strings.TrimSpace(string(out))
	env := append(b.environ(), "GIT_DIR="+gitDir, "GIT_INDEX_FILE="+filepath.Join(gitDir, "index"))
	b.environ = func() []string { return env }
	return b
}

// One hooks directory several repositories share, as a core.hooksPath that
// all of them read makes it, arms only the repository setup ran in and
// its linked worktrees, also as git runs a hook there: init alone never
// arms a rung (decision 10, A11), and a receipt of another repository
// would refuse a commit in one the person never set up.
func TestArmedBySetupArmsOnlyTheRepositorySetupRanIn(t *testing.T) {
	t.Parallel()
	b, home := testBinding(t)
	hooks := filepath.Join(home, "team-hooks")
	if err := os.MkdirAll(hooks, 0o700); err != nil {
		t.Fatal(err)
	}
	set, other := repository(t, true), repository(t, true)
	for _, root := range []string{set, other} {
		gitRun(t, root, "config", "core.hooksPath", hooks)
	}
	receiptOf(t, b, set, hooks)
	linked, otherLinked := filepath.Join(home, "linked tree"), filepath.Join(home, "other tree")
	gitRun(t, set, "worktree", "add", "-q", "-b", "linked", linked)
	gitRun(t, other, "worktree", "add", "-q", "-b", "linked", otherLinked)
	cases := []struct {
		name    string
		binding binding
		dir     string
		want    bool
	}{
		{"the repository setup ran in", b, set, true},
		{"its linked worktree", b, linked, true},
		{"its linked worktree as git runs the hook there", asHook(t, b, linked), linked, true},
		{"another repository of the shared hooks directory", b, other, false},
		{"a linked worktree of that one as git runs the hook there", asHook(t, b, otherLinked), otherLinked, false},
	}
	for _, c := range cases {
		if got := c.binding.armedBySetup(t.Context(), c.dir); got != c.want {
			t.Errorf("%s: armed = %v, want %v", c.name, got, c.want)
		}
	}
}

// What the binding cannot read arms nothing, so the rung answers nothing
// and the pipeline judges the commit: a state directory it cannot resolve,
// a directory git knows no repository of, a receipt of another hooks
// directory and one whose worktree is gone.
func TestArmedBySetupAnswersNothingItCannotRead(t *testing.T) {
	t.Parallel()
	b, home := testBinding(t)
	root := repository(t, true)
	gone := filepath.Join(home, "gone tree")
	gitRun(t, root, "worktree", "add", "-q", "-b", "gone", gone)
	receiptOf(t, b, gone, filepath.Join(root, ".git", "hooks"))
	gitRun(t, root, "worktree", "remove", gone)
	elsewhere := repository(t, true)
	receiptOf(t, b, elsewhere, filepath.Join(elsewhere, ".git", "hooks"))
	unresolved := b
	unresolved.env = func(string) string { return "" }
	for name, armed := range map[string]bool{
		"a state directory the binding cannot resolve":                        unresolved.armedBySetup(t.Context(), root),
		"a directory of no repository":                                        b.armedBySetup(t.Context(), home),
		"a receipt of another hooks directory and one whose worktree is gone": b.armedBySetup(t.Context(), root),
	} {
		if armed {
			t.Errorf("%s armed the rung", name)
		}
	}
	if !b.armedBySetup(t.Context(), elsewhere) {
		t.Error("the receipt of its own repository did not arm the rung")
	}
}

// A hooks directory git names but nobody made yet compares by its clean
// form, so the receipt that names it still arms the rung.
func TestArmedBySetupComparesAHooksDirectoryNotMadeByItsCleanForm(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := repository(t, true)
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	hooks := filepath.Join(base, "hooks not made")
	gitRun(t, root, "config", "core.hooksPath", hooks)
	receiptOf(t, b, root, hooks+string(filepath.Separator)+".")
	if !b.armedBySetup(t.Context(), root) {
		t.Fatal("the receipt of a hooks directory not made yet did not arm the rung")
	}
}
