package main

import (
	"context"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// The on-demand formatter of golaw.mk rewrites the tree golangci-lint fmt
// and lint read in the pre-commit rung, so it takes the lock the rung
// takes: a make fmt started while a commit runs its toolchain waits for it,
// and a commit started during a make fmt waits for the formatter.
//
//nolint:paralleltest // The command resolves its repository from cwd.
func TestMakeFmtTakesTheLockOfThePreCommitRung(t *testing.T) {
	data, err := fs.ReadFile(golaw.FS, "templates/golaw.mk.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	_, recipe, found := strings.Cut(string(data), "\nfmt:\n\t")
	recipe, _, _ = strings.Cut(recipe, "\n")
	fields := strings.Fields(recipe)
	if !found || len(fields) < 4 || fields[0] != "golaw" || fields[1] != "lock" || fields[3] != "--" {
		t.Fatalf("the fmt recipe runs no golaw lock: %q", recipe)
	}
	if fields[2] != rung.PreCommit {
		t.Fatalf("make fmt takes the lock %q and the pre-commit rung %q", fields[2], rung.PreCommit)
	}
	repo := gittest.New(t)
	t.Chdir(repo.Root)
	held, err := runner.Acquire(t.Context(), runner.LockPath(repo.GitDir(), rung.PreCommit), 0)
	if err != nil {
		t.Fatal(err)
	}
	program := runnertest.Install(t, t.TempDir(), "probe", runnertest.Behavior{Probe: []string{"version"}})
	notice := &releasingNotice{release: held.Release}
	ta := ladderApp(t)
	ta.app.stderr = notice
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if got := ta.run(ctx, []string{"lock", fields[2], "--", program}); got != exitClean || notice.release != nil {
		t.Fatalf("exit = %d, notice = %q: the formatter did not wait for the lock the rung holds", got, notice.output.String())
	}
}
