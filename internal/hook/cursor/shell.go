package cursor

import (
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// shellRequest is the request of a beforeShellExecution event: its
// command, judged by the shell family where the envelope says it runs, as
// preToolUse Shell judges it (decision 13 of the plan); the two differ in
// identity only, this one keeping no baseline because it carries no tool
// use.
func shellRequest(input payload, event model.Event) (hook.Request, error) {
	command, err := input.required(commandField)
	if err != nil {
		return hook.Request{}, err
	}
	return shellCommand(event, command), nil
}

// shellCommand is the plain shell request of a command under the Shell
// tool with the identities the header read: the core judges it and keeps
// the baseline of an allowed one when the event carries a tool use.
func shellCommand(event model.Event, command string) hook.Request {
	return hook.Request{
		Event: model.Event{
			Shape:     model.ShapeShell,
			Agent:     agentID,
			Tool:      shellTool,
			Cwd:       event.Cwd,
			SessionID: event.SessionID,
			AgentID:   event.AgentID,
			ToolUseID: event.ToolUseID,
			Shell: &model.ShellCommand{
				Command: command,
				Cwd:     event.Cwd,
				Tool:    shellTool,
			},
		},
	}
}
