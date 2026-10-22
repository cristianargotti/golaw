package cursor

import (
	"fmt"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// decodeRequest is the request of an enrolled event, selected by the
// event name and the tool the header read: each route decodes one
// documented payload, and every other event or tool cannot be judged,
// never allowed by a verdict the policy did not give.
func decodeRequest(input payload, event model.Event, _ hook.Base, _ model.Snapshot) (hook.Request, error) {
	name := input.text(nameField)
	switch name {
	case shellEvent:
		return shellRequest(input, event)
	case preEvent:
		return preRequest(input, event)
	default:
		return hook.Request{}, fmt.Errorf("%w: unsupported cursor event %s", model.ErrCannotJudge, name)
	}
}

// preRequest is the request of a preToolUse event by its tool: a Shell
// command is judged by the shell family as beforeShellExecution judges it
// (decision 13 of the plan); a Write or Delete is captured and allowed
// while writeEnforced is false (D8), and once it is true the route cannot
// judge until a typed decoder exists, so the switch alone never invents
// enforcement.
func preRequest(input payload, event model.Event) (hook.Request, error) {
	switch event.Tool {
	case shellTool:
		return observeShell(input, event)
	case writeTool, deleteTool:
		if writeEnforced {
			return hook.Request{}, fmt.Errorf("%w: cursor %s has no typed decoder yet", model.ErrCannotJudge, event.Tool)
		}
		return captureRequest(input, event)
	default:
		return hook.Request{}, fmt.Errorf("%w: unsupported cursor tool %s", model.ErrCannotJudge, event.Tool)
	}
}
