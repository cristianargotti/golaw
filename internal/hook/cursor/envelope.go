package cursor

import (
	"encoding/json"
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

// The event names Cursor documents and the tools the adapter names: the
// read and shell events carry no tool name, so the adapter gives them the
// one the core judges them under.
const (
	preEvent     = "preToolUse"
	postEvent    = "postToolUse"
	failureEvent = "postToolUseFailure"
	readEvent    = "beforeReadFile"
	shellEvent   = "beforeShellExecution"
	writeTool    = "Write"
	deleteTool   = "Delete"
	shellTool    = "Shell"
	readTool     = "Read"
	agentID      = "cursor"
)

// The fields of the envelope more than one route reads.
const (
	nameField = "hook_event_name"
	toolField = "tool_name"
)

// payload is the envelope of a Cursor event with every field kept as it
// came, so an absent field is told from a null one and a field a later
// Cursor adds is tolerated (risk 2).
type payload map[string]json.RawMessage

// parsePayload decodes the envelope of an event once and refuses one that
// is no object, one that carries a field of another agent (D16) and one
// without the identity and the roots every Cursor event carries.
func parsePayload(data []byte) (payload, error) {
	var input payload
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("%w: cursor envelope: %w", model.ErrCannotJudge, err)
	}
	if input == nil {
		return nil, fmt.Errorf("%w: cursor envelope must be one object", model.ErrCannotJudge)
	}
	if _, foreign := input["turn_id"]; foreign {
		return nil, fmt.Errorf("%w: envelope does not belong to cursor", model.ErrCannotJudge)
	}
	if input.text(nameField) == "" || input.text("conversation_id") == "" {
		return nil, fmt.Errorf("%w: incomplete cursor envelope", model.ErrCannotJudge)
	}
	if _, err := input.roots(); err != nil {
		return nil, err
	}
	return input, nil
}

// text is the string a field holds, empty when the field is absent or
// holds another type; a route that needs the difference asks required.
func (input payload) text(name string) string {
	var value string
	if raw, present := input[name]; present {
		if err := json.Unmarshal(raw, &value); err != nil {
			return ""
		}
	}
	return value
}

// required is a field a route cannot do without: absent or of another
// type, the event cannot be judged.
func (input payload) required(name string) (string, error) {
	raw, present := input[name]
	if !present {
		return "", fmt.Errorf("%w: cursor %s is absent", model.ErrCannotJudge, name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%w: cursor %s is no string: %w", model.ErrCannotJudge, name, err)
	}
	return value, nil
}

// roots are the workspace roots of the event, an array of strings that
// may be empty (D7); a null is no array, so it is refused with the other
// types.
func (input payload) roots() ([]string, error) {
	raw, present := input["workspace_roots"]
	if !present || len(raw) == 0 || raw[0] != '[' {
		return nil, fmt.Errorf("%w: incomplete cursor envelope: workspace_roots is no array", model.ErrCannotJudge)
	}
	var roots []string
	if err := json.Unmarshal(raw, &roots); err != nil {
		return nil, fmt.Errorf("%w: cursor workspace_roots: %w", model.ErrCannotJudge, err)
	}
	return roots, nil
}

// eventHeader is the event of an envelope before its request is decoded:
// the A15 identity of D7 (the conversation as the session, the generation
// as the agent id, the tool use as given), the working directory of the
// payload or else the first workspace root, never the process's, and the
// shape its event name gives it. Enrollment discovers the repository
// from the event, so no root string is hashed here.
func eventHeader(input payload) (model.Event, error) {
	roots, err := input.roots()
	if err != nil {
		return model.Event{}, err
	}
	event := model.Event{
		Agent: agentID, Tool: input.text(toolField), Cwd: input.text("cwd"),
		SessionID: input.text("conversation_id"), AgentID: input.text("generation_id"),
		ToolUseID: input.text("tool_use_id"),
	}
	if event.Cwd == "" && len(roots) > 0 {
		event.Cwd = roots[0]
	}
	return projectShape(event, input.text(nameField))
}

// projectShape gives an event the shape and the tool its documented name
// selects, so enrollment tells a read from a write and the encoder keeps
// the documented answer of a read or post event whose decoding fails
// later; a tool preToolUse does not judge keeps no shape and the decoder
// refuses it.
func projectShape(event model.Event, name string) (model.Event, error) {
	switch name {
	case preEvent:
		event.Shape = preShape(event.Tool)
	case postEvent, failureEvent:
		event.Shape = model.ShapePost
		event.Post = &model.PostResult{Failed: name == failureEvent}
	case readEvent:
		event.Shape, event.Tool = model.ShapeRead, readTool
	case shellEvent:
		event.Shape, event.Tool = model.ShapeShell, shellTool
	default:
		return model.Event{}, fmt.Errorf("%w: unsupported cursor event %s", model.ErrCannotJudge, name)
	}
	return event, nil
}

// preShape is the shape of a preToolUse event by its tool: the write
// policy for Write and Delete, the shell policy for Shell.
func preShape(tool string) model.EventShape {
	switch tool {
	case writeTool, deleteTool:
		return model.ShapeWrite
	case shellTool:
		return model.ShapeShell
	default:
		return 0
	}
}
