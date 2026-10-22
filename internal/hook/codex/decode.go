package codex

import (
	"encoding/json"
	"fmt"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// toolInput is the tool input of an event decoded once: the command both
// canonical tools carry in tool_input.command, with what the decoding met,
// which the decoder answers once the event is enrolled, so enrollment
// precedes every reconstruction, and the tool_response of a post event.
type toolInput struct {
	command  *string
	err      error
	response json.RawMessage
}

// readInput decodes the tool input of a Bash or apply_patch event once; the
// input of any other tool stays undecoded, because its route is refused.
func readInput(w wire) toolInput {
	if w.Tool != bashTool && w.Tool != patchTool {
		return toolInput{}
	}
	var shell struct {
		Command *string `json:"command"`
	}
	in := toolInput{err: json.Unmarshal(w.Input, &shell), response: w.Response}
	in.command = shell.Command
	return in
}

// text is the command of a tool input, which both canonical tools carry as
// a required string.
func (in toolInput) text(tool string) (string, error) {
	if in.err != nil {
		return "", fmt.Errorf("%w: %s input: %w", model.ErrCannotJudge, tool, in.err)
	}
	if in.command == nil {
		return "", fmt.Errorf("%w: %s command is absent", model.ErrCannotJudge, tool)
	}
	return *in.command, nil
}

// decode is the request of an event from its one parse: a tool outside the
// two canonical ones is refused here, after enrollment, so an unenrolled
// location keeps its valid allow.
func decode(event model.Event, in toolInput, base hook.Base, snapshot model.Snapshot) (hook.Request, error) {
	if event.Tool != bashTool && event.Tool != patchTool {
		return hook.Request{}, fmt.Errorf("%w: unsupported Codex tool %s", model.ErrCannotJudge, event.Tool)
	}
	command, err := in.text(event.Tool)
	if err != nil {
		return hook.Request{}, err
	}
	switch {
	case event.Shape == model.ShapePost:
		return postRequest(event, command, in.response, base)
	case event.Tool == patchTool:
		return patchRequest(event, command, base, snapshot)
	default:
		return shellRequest(event, command, base, snapshot)
	}
}

// shellRequest normalizes a Bash pre event: the command text as the shell
// family reads it, where it runs and the tool name Codex writes, with the
// files a recognized here-document form writes attached as its writes,
// which the core judges before the command.
func shellRequest(event model.Event, command string, base hook.Base, snapshot model.Snapshot) (hook.Request, error) {
	event.Shape = model.ShapeShell
	event.Shell = &model.ShellCommand{Command: command, Cwd: event.Cwd, Tool: bashTool}
	request := hook.Request{Event: event}
	form, ok := recognize(command)
	switch {
	case !ok:
		return request, nil
	case form.patch:
		return heredocPatch(request, form, base, snapshot)
	default:
		return heredocCat(request, form, base, snapshot)
	}
}
