package main

import (
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// headOf answers the commit HEAD names in the repository at root.
func headOf(t *testing.T, root string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", "rev-parse", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// The armed pre-push rung runs govulncheck alone over the pushed tip, in the
// form lefthook runs with the remote and its URL and in the form section
// 4.4 lists with neither, against a repository whose remote is origin. It
// is serial: the fake govulncheck is found through PATH, and the rung runs
// in the directory t.Chdir sets.
func TestHookRunsThePrePushRungOfTheRepository(t *testing.T) {
	root := repository(t, true)
	remote := t.TempDir()
	gitRun(t, remote, "init", "-q", "--bare")
	gitRun(t, root, "remote", "add", "origin", remote)
	gitRun(t, root, "push", "-q", "origin", "HEAD:refs/heads/main")
	gitRun(t, root, "fetch", "-q", "origin")
	tip := headOf(t, root)
	fakes := t.TempDir()
	runnertest.Install(t, fakes, "govulncheck", runnertest.Behavior{Stdout: `{"config": {"scanner_name": "govulncheck", "scanner_version": "v1.7.0"}}` + "\n"})
	t.Setenv("PATH", fakes+string(os.PathListSeparator)+os.Getenv("PATH"))
	want := "golaw pre-push: " + tip[:12] + ": govulncheck finds no reachable vulnerability\n" //nolint:gocritic // a commit hash is hex, ASCII by construction
	for _, args := range [][]string{{"hook", "pre-push", "origin", remote}, {"hook", "pre-push"}} {
		ta := rungApp(t, root)
		ta.stdin = strings.NewReader("refs/heads/side " + tip + " refs/heads/side " + runner.ZeroSHA + "\n")
		if got := ta.run(t.Context(), args); got != exitClean || ta.stdout.String() != want || ta.stderr.Len() != 0 {
			t.Errorf("%v: exit %d, stdout %q, stderr %q", args, got, ta.stdout.String(), ta.stderr.String())
		}
	}
	if calls := runnertest.Calls(t, fakes, "govulncheck"); len(calls) != 2 {
		t.Errorf("govulncheck calls = %q", calls)
	}
	// The rung reads the remote only for the base of a new ref, which no
	// line shows, so the default remote is pinned where the arguments are
	// read.
	if args, err := rungArguments(rung.PrePush, nil); err != nil || !slices.Equal(args, []string{"origin", ""}) {
		t.Errorf("pre-push without arguments reads %q, %v", args, err)
	}
}
