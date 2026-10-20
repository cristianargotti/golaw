package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/rung"
)

// hookArgs is the most arguments hook takes: the name of pre-push, its
// remote and its URL.
const hookArgs = 3

// defaultRemote is the remote of a pre-push rung run without arguments, as
// section 4.4 lists hook pre-push.
const defaultRemote = "origin"

// rungArgs is how many arguments a git rung takes after its name: the
// message file of commit-msg, the remote name and URL of pre-push, as
// lefthook passes them with {1} and {2}.
func rungArgs(name string) int {
	switch name {
	case rung.CommitMsg:
		return 1
	case rung.PrePush:
		return 2
	default:
		return 0
	}
}

// rungArguments checks the arguments of a rung after its name; pre-push
// also runs with none, as section 4.4 lists it, and reads the remote
// origin then.
func rungArguments(name string, args []string) ([]string, error) {
	if name == rung.PrePush && len(args) == 0 {
		return []string{defaultRemote, ""}, nil
	}
	if len(args) != rungArgs(name) {
		return nil, fmt.Errorf("%w: hook %s takes %d arguments after its name, got %d", model.ErrUsage, name, rungArgs(name), len(args))
	}
	return args, nil
}

// runRung runs one git rung in the directory git runs the hook in, under
// the interrupt and terminate signals, so a person who stops the commit or
// the push stops the rung and the copy of a tree it made is removed. A rung
// golaw setup did not arm answers nothing. Its lines leave on stdout and
// its code as an exitError after them.
func (a *app) runRung(ctx context.Context, out io.Writer, name string, args []string) error {
	args, err := rungArguments(name, args)
	if err != nil {
		return hookFailed(err)
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	dir, err := os.Getwd()
	if err != nil {
		return hookFailed(fmt.Errorf("find the directory of the hook: %w", err))
	}
	if !a.bind().armedBySetup(ctx, dir) {
		return nil
	}
	registry, err := a.registry()
	if err != nil {
		return hookFailed(err)
	}
	deps, err := a.bind().rungDependencies(dir, a.now(), a.stdin, registry)
	if err != nil {
		return hookFailed(err)
	}
	var outcome rung.Outcome
	switch name {
	case rung.CommitMsg:
		outcome = rung.RunCommitMsg(ctx, deps, args[0])
	case rung.PrePush:
		outcome = rung.RunPrePush(ctx, deps, args[0], args[1])
	default:
		outcome = rung.RunPreCommit(ctx, deps)
	}
	if err := writeLines(out, outcome.Lines); err != nil {
		return hookFailed(err)
	}
	if outcome.Code != exitClean {
		return exitError{code: outcome.Code}
	}
	return nil
}
