package main

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/install"
)

// agentAll is the --agent value that runs an installer for every agent of
// the registry (specification section 4.3).
const agentAll = "all"

// everyAgent runs one agent's command, or with --agent all every agent's
// in id order, each under a line naming it, stopping at the first failure,
// so the agent that failed is the last one named.
func (a *app) everyAgent(run agentRun) agentRun {
	return func(ctx context.Context, out io.Writer, path string, opts agentOptions) error {
		if opts.agent != agentAll {
			return run(ctx, out, path, opts)
		}
		for _, id := range agents.IDs() {
			one := opts
			one.agent, one.every = id, true
			if err := writeLines(out, []string{"agent: " + id}); err != nil {
				return err
			}
			if err := run(ctx, out, path, one); err != nil {
				return err
			}
		}
		return nil
	}
}

// uninstallInstalled removes an agent named alone as agentUninstall does;
// under --agent all an agent without a receipt for the scope is skipped
// with a line, because there is nothing of it to remove.
func (a *app) uninstallInstalled(ctx context.Context, out io.Writer, path string, opts agentOptions) error {
	agent, root, err := a.agentTarget(ctx, path, opts)
	if err != nil {
		return uninstallFailed(err)
	}
	receipt, err := a.receiptPath(agent, root, opts.scope)
	if err != nil {
		return uninstallFailed(err)
	}
	if _, statErr := os.Stat(receipt); errors.Is(statErr, fs.ErrNotExist) && opts.every {
		return uninstallFailed(writeLines(out, []string{"not installed: " + agent.ID}))
	}
	return a.agentUninstall(ctx, out, path, opts)
}

// installedLines closes an installation: the receipt it wrote, then the
// notes its planner leaves the person, such as a trust step to take.
func installedLines(plan install.Plan) []string {
	return append([]string{"installed: " + plan.ReceiptPath}, plan.Notes...)
}
