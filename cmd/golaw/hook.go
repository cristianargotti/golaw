package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/model"
)

// hookCommand answers one hook event of an agent: the JSON object on stdin
// is judged through the engine and the answer of the agent's contract
// leaves on stdout and stderr with its exit code (specification section
// 8.2). The agent ids come from the hook packages, never from this file.
func (a *app) hookCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "hook <agent>",
		Short: "Answer one hook event of an agent, read as JSON on stdin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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

// lookupAgent resolves an agent id or names the known ones.
func lookupAgent(id string) (agents.Agent, error) {
	agent, ok := agents.Lookup(id)
	if !ok {
		return agents.Agent{}, fmt.Errorf("%w: unknown agent %q, known: %s", model.ErrUsage, id, strings.Join(agents.IDs(), ", "))
	}
	return agent, nil
}

// hookFailed names the command in front of a failure.
func hookFailed(err error) error {
	return fmt.Errorf("hook: %w", err)
}
