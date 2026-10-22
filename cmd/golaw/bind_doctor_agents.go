package main

import (
	"fmt"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/hook/agents"
)

// agentDirs fills what the report of an agent names beyond the evidence:
// the agent and the scope, the directory its host reads its settings from,
// the home of the person and the state directory receipts, observations
// and captures share.
func (b binding) agentDirs(agent agents.Agent, scope string, input *doctor.Input) error {
	home, err := b.home()
	if err != nil {
		return fmt.Errorf("resolve the home directory: %w", err)
	}
	state, err := b.installState()
	if err != nil {
		return err
	}
	input.Agent, input.Scope, input.Home, input.StateDir = agent.ID, scope, home, state
	input.ConfigDir = agent.ConfigDir(b.env, home)
	return nil
}
