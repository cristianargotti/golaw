// Package gittest builds real git repositories under the temporary
// directory of a test, for the tests of every package that reads git: an
// isolated configuration, a fixed identity, none of the variables a
// surrounding git gives a hook, and the states the staged rungs read.
package gittest

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The identity every helper commits as.
const (
	Name  = "tester"
	Email = "tester@example.com"
)

// The git words more than one helper speaks.
const (
	head     = "HEAD"
	revParse = "rev-parse"
)

// Isolated is the environment every helper runs git with: the environment
// a git command would inherit without any GIT_ variable, so a hook that
// runs the tests never points them at its own repository, with the fixed
// identity as author and committer, no global or system configuration and
// line endings kept as written.
func Isolated() []string {
	inherited := (&exec.Cmd{}).Environ()
	out := make([]string, 0, len(inherited))
	for _, entry := range inherited {
		if !strings.HasPrefix(strings.ToUpper(entry), "GIT_") {
			out = append(out, entry)
		}
	}
	return append(out,
		"GIT_AUTHOR_NAME="+Name, "GIT_AUTHOR_EMAIL="+Email,
		"GIT_COMMITTER_NAME="+Name, "GIT_COMMITTER_EMAIL="+Email,
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.autocrlf", "GIT_CONFIG_VALUE_0=false",
	)
}

// Repo is a real repository a test owns.
type Repo struct {
	Root string // the real path of the worktree root

	t testing.TB
}

// New creates an empty repository on branch main under the temporary
// directory of tb, at its real path, so a path git answers compares equal.
func New(tb testing.TB) *Repo {
	tb.Helper()
	r := &Repo{Root: realDir(tb), t: tb}
	r.Git("init", "-q", "-b", "main")
	return r
}

// Git runs git in the worktree root with Isolated and answers its output
// without the surrounding whitespace; a failure fails the test.
func (r *Repo) Git(args ...string) string {
	r.t.Helper()
	return r.GitEnv(nil, args...)
}

// GitEnv is Git with variables added after Isolated, which win over it.
func (r *Repo) GitEnv(extra []string, args ...string) string {
	r.t.Helper()
	out, err := r.run(extra, args...)
	if err != nil {
		r.t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return out
}

// Head answers the HEAD commit id, empty on an unborn branch.
func (r *Repo) Head() string {
	r.t.Helper()
	out, err := r.run(nil, revParse, "--verify", "-q", head)
	if err != nil {
		return ""
	}
	return out
}

// GitDir answers the absolute git directory of this worktree.
func (r *Repo) GitDir() string {
	r.t.Helper()
	return r.Git(revParse, "--absolute-git-dir")
}

// run is one git command; its error carries what git printed.
func (r *Repo) run(extra []string, args ...string) (string, error) {
	cmd := exec.CommandContext(r.t.Context(), "git", args...) //nolint:gosec // the arguments are the test's own
	cmd.Dir = r.Root
	cmd.Env = append(Isolated(), extra...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

// realDir is a new temporary directory of tb at its real path.
func realDir(tb testing.TB) string {
	tb.Helper()
	dir, err := filepath.EvalSymlinks(tb.TempDir())
	if err != nil {
		tb.Fatalf("resolve the temporary directory: %v", err)
	}
	return dir
}
