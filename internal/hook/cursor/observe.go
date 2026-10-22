package cursor

import (
	"encoding/json"
	"fmt"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// observeShell is the request of a preToolUse Shell event: the command of
// its object tool_input as a plain shell request, so the core judges it
// through the shell family with the reason beforeShellExecution gives the
// same command and any denial wins whichever event a Cursor build fires
// (decision 13 of the plan), and keeps the baseline of an allowed one
// under the tool use identity for the post events of the same call (A15).
// The working directory is the envelope's (D7): the nested
// working_directory is not read.
func observeShell(input payload, event model.Event) (hook.Request, error) {
	var toolInput payload
	if err := json.Unmarshal(input["tool_input"], &toolInput); err != nil {
		return hook.Request{}, inputError(err)
	}
	if toolInput == nil {
		return hook.Request{}, fmt.Errorf("%w: cursor tool_input is no object", model.ErrCannotJudge)
	}
	command, err := toolInput.required(commandField)
	if err != nil {
		return hook.Request{}, err
	}
	return shellCommand(event, command), nil
}
