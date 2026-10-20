package main

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/rung"
)

// hookCommand answers one hook event of an agent: the JSON object on stdin
// is judged through the engine and the answer of the agent's contract
// leaves on stdout and stderr with its exit code (specification section
// 8.2). It also runs the three git rungs of section 7, one binary call each:
// hook pre-commit, hook commit-msg <file> and hook pre-push [<remote> <url>].
// The agent ids and the rung names come from their packages, never from
// this file.
func (a *app) hookCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "hook <agent> | pre-commit | commit-msg <file> | pre-push [<remote> <url>]",
		Short: "Answer one hook event of an agent, read as JSON on stdin, or run a git rung",
		Args:  cobra.RangeArgs(1, hookArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			if slices.Contains(rung.Names(), args[0]) {
				return a.runRung(cmd.Context(), cmd.OutOrStdout(), args[0], args[1:])
			}
			if len(args) != 1 {
				return hookFailed(fmt.Errorf("%w: hook %s takes no argument after the agent", model.ErrUsage, args[0]))
			}
			return a.hook(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), args[0])
		},
	}
}

// hook runs the bounded evaluation and relays the reply. A reply with an
// exit code other than zero is a result the adapter already printed, so it
// leaves as an exitError without a second message.
func (a *app) hook(ctx context.Context, out, errOut io.Writer, id string) error {
	agent, err := lookupAgent(id)
	if err != nil {
		return hookFailed(err)
	}
	deps, err := a.hookDeps(agent, a.now())
	if err != nil {
		return hookFailed(err)
	}
	reply := hook.Run(ctx, agent.ID, a.stdin, agent.Adapter, deps)
	if _, err := out.Write(reply.Stdout); err != nil {
		return hookFailed(fmt.Errorf("write the answer: %w", err))
	}
	if _, err := io.WriteString(errOut, reply.Stderr); err != nil {
		return hookFailed(fmt.Errorf("write the diagnostic: %w", err))
	}
	if reply.ExitCode != 0 {
		return exitError{code: reply.ExitCode}
	}
	return nil
}

func lookupAgent(id string) (agents.Agent, error) {
	agent, ok := agents.Lookup(id)
	if !ok {
		return agents.Agent{}, fmt.Errorf("%w: unknown agent %q, known: %s", model.ErrUsage, id, strings.Join(agents.IDs(), ", "))
	}
	return agent, nil
}

func hookFailed(err error) error {
	return fmt.Errorf("hook: %w", err)
}
