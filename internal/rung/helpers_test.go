package rung_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/source"
)

// now is the clock the rungs of these tests read.
var now = time.Date(2026, time.October, 19, 12, 0, 0, 0, time.UTC)

// armedPolicy pins the kit, enables go and arms every rung; a test adds
// keys after it.
const armedPolicy = "golaw: 1\nkit: {version: 0.1.0}\ncapabilities: [go]\n"

// armedRepo commits a module and the policy, and answers the repository.
func armedRepo(t *testing.T, golaw string) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	r.Commit("build: start the module", map[string]string{"go.mod": "module example.test/m\n\ngo 1.27\n", policy.FileName: golaw})
	return r
}

// deps are the dependencies cmd binds, over the repository: its root, the
// environment of a git without hook variables, the production catalog, a
// development build that skips the pin and a state directory of the test.
func deps(t *testing.T, r *gittest.Repo, extra ...string) rung.Dependencies {
	t.Helper()
	registry, err := checks.Registry()
	if err != nil {
		t.Fatal(err)
	}
	state := t.TempDir()
	load := func(ctx context.Context, dir string) (*policy.Loaded, error) {
		discover := func(ctx context.Context, dir string) (policy.Discovery, error) {
			repo, err := source.Discover(ctx, dir)
			if err != nil {
				return policy.Discovery{}, fmt.Errorf("discover: %w", err)
			}
			return policy.Discovery{Root: repo.Root, Head: repo.Head}, nil
		}
		loaded, err := policy.Load(ctx, policy.Input{Dir: dir, Dirs: policy.Dirs{State: state, Config: state}, Now: now, Discover: discover})
		if err != nil {
			return nil, fmt.Errorf("load the policy of %s: %w", dir, err)
		}
		return loaded, nil
	}
	return rung.Dependencies{
		Dir: r.Root, Environ: append(gittest.Isolated(), extra...), Now: now, Load: load,
		Registry: registry, State: state, Stdin: strings.NewReader(""),
	}
}

// message writes the message file where git writes it and answers the
// path git hands the hook, relative to the worktree root.
func message(r *gittest.Repo, text string) string {
	r.Write(".git/COMMIT_EDITMSG", text)
	return ".git/COMMIT_EDITMSG"
}

// files are n new files of one line each.
func files(n int) map[string]string {
	out := make(map[string]string, n)
	for i := range n {
		out[fmt.Sprintf("tables/f%02d.txt", i+1)] = "line\n"
	}
	return out
}

// contains reports whether a line of the outcome holds the text.
func contains(out rung.Outcome, text string) bool {
	for _, line := range out.Lines {
		if strings.Contains(line, text) {
			return true
		}
	}
	return false
}
