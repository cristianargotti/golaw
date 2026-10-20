package main

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// realLefthook skips a case of the real lefthook where the pinned one is
// not on PATH, which CI installs before make ci, and fails it in CI.
func realLefthook(t *testing.T) {
	t.Helper()
	tool, ok := runner.LookupTool("lefthook")
	if !ok {
		t.Fatal("lefthook is not a pinned tool")
	}
	if _, err := runner.Verify(t.Context(), tool, os.Environ()); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("CI installs lefthook %s before the tests: %v", tool.Version, err)
		}
		t.Skipf("lefthook %s is not on PATH, which CI installs: %v", tool.Version, err)
	}
}

// lefthookEnv is the environment of the git and the lefthook of a case: the
// process environment without the variables that switch lefthook off or
// point it at another configuration, without the hook variables of a git
// that started the tests, and without a global or system git
// configuration, whose core.hooksPath would move the hooks.
func lefthookEnv() []string {
	var env []string
	for _, entry := range runner.UserEnv(os.Environ()) {
		if name, _, _ := strings.Cut(entry, "="); !strings.HasPrefix(strings.ToUpper(name), "LEFTHOOK") {
			env = append(env, entry)
		}
	}
	return append(env, "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
}

// inRepo runs git or lefthook in dir under lefthookEnv and fails the test
// with what it printed when it fails.
func inRepo(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), name, args...)
	cmd.Dir, cmd.Env = dir, lefthookEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

// lefthookConfigs are the two lefthook.yml files of the kit: its own, and
// the one init writes into a repository.
func lefthookConfigs(t *testing.T) map[string][]byte {
	t.Helper()
	own, err := os.ReadFile(filepath.Join("..", "..", "lefthook.yml"))
	if err != nil {
		t.Fatal(err)
	}
	written, err := fs.ReadFile(golaw.FS, "templates/lefthook.yml.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	return map[string][]byte{"the kit's own lefthook.yml": own, "the lefthook.yml init writes": written}
}

// spacedRepository commits the lefthook.yml config in a repository under a
// path with a space, arms it with lefthook install and answers its root.
func spacedRepository(t *testing.T, config []byte) string {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "my repo")
	inRepo(t, base, "git", "init", "-q", root)
	inRepo(t, root, "git", "config", "user.email", "tester@example.test")
	inRepo(t, root, "git", "config", "user.name", "tester")
	if err := os.WriteFile(filepath.Join(root, "lefthook.yml"), config, 0o600); err != nil {
		t.Fatal(err)
	}
	inRepo(t, root, "git", "add", "lefthook.yml")
	inRepo(t, root, "git", "commit", "-q", "-m", "build: run the rungs")
	inRepo(t, root, "lefthook", "install")
	return root
}

// lefthook writes a git argument into the shell line it runs, so a path
// with a space reached golaw hook as two arguments: a commit in a linked
// worktree of a repository under such a path, whose message file git
// names by its absolute path, and a push to a remote at such a path were
// refused with a usage error the person could not act on. Each git
// argument of both files reaches golaw whole.
//
//nolint:paralleltest // the fake golaw is found through PATH
func TestLefthookHandsGolawEachGitArgumentWhole(t *testing.T) {
	realLefthook(t)
	path := os.Getenv("PATH")
	for name, config := range lefthookConfigs(t) {
		bin := t.TempDir()
		runnertest.Install(t, bin, "golaw", runnertest.Behavior{})
		t.Setenv("PATH", bin+string(os.PathListSeparator)+path)
		root := spacedRepository(t, config)
		worktree := filepath.Join(filepath.Dir(root), "side tree")
		inRepo(t, root, "git", "worktree", "add", "-q", "-b", "side", worktree)
		inRepo(t, worktree, "git", "commit", "-q", "--allow-empty", "-m", "docs: note the side")
		remote := filepath.ToSlash(filepath.Join(filepath.Dir(root), "remote space.git"))
		inRepo(t, root, "git", "init", "-q", "--bare", remote)
		inRepo(t, root, "git", "remote", "add", "spaced", remote)
		inRepo(t, root, "git", "push", "-q", "spaced", "HEAD:refs/heads/main")
		calls := runnertest.Calls(t, bin, "golaw")
		if len(calls) != 2 || len(calls[0]) != 3 || calls[0][0] != "hook" || calls[0][1] != "commit-msg" ||
			!strings.Contains(filepath.ToSlash(calls[0][2]), "/my repo/") {
			t.Fatalf("%s: golaw calls = %q", name, calls)
		}
		if message, err := os.ReadFile(calls[0][2]); err != nil || !strings.HasPrefix(string(message), "docs: note the side") {
			t.Errorf("%s: the message file %q holds %q, %v", name, calls[0][2], message, err)
		}
		if want := []string{"hook", "pre-push", "spaced", remote}; !slices.Equal(calls[1], want) {
			t.Errorf("%s: the push called golaw with %q, want %q", name, calls[1], want)
		}
	}
}
