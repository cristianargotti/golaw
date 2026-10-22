package claude

import (
	"encoding/json"
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	preEvent     = "PreToolUse"
	postEvent    = "PostToolUse"
	failureEvent = "PostToolUseFailure"
	writeTool    = "Write"
	editTool     = "Edit"
	notebookTool = "NotebookEdit"
	readTool     = "Read"
	bashTool     = "Bash"
	powerTool    = "PowerShell"
)

// wire is the envelope of a Claude event. The four fields after the tool
// fields belong only to the envelope of another agent: any of them, null
// included, refuses the event.
type wire struct {
	Name     string          `json:"hook_event_name"`
	Tool     string          `json:"tool_name"`
	Cwd      string          `json:"cwd"`
	Session  string          `json:"session_id"`
	Agent    string          `json:"agent_id"`
	Use      string          `json:"tool_use_id"`
	Input    json.RawMessage `json:"tool_input"`
	Response json.RawMessage `json:"tool_response"`
	Error    json.RawMessage `json:"error"`

	Conversation json.RawMessage `json:"conversation_id"`
	Generation   json.RawMessage `json:"generation_id"`
	Roots        json.RawMessage `json:"workspace_roots"`
	Version      json.RawMessage `json:"cursor_version"`
}

// header is the event of an envelope before any target is reconstructed:
// its identities, the shape its event and tool give it and the path its
// tool input names.
func header(w wire, target string) (model.Event, error) {
	event := model.Event{
		Agent: "claude", Tool: w.Tool, Cwd: w.Cwd, Target: target,
		SessionID: w.Session, AgentID: w.Agent, ToolUseID: w.Use,
	}
	switch w.Name {
	case preEvent:
		event.Shape = preShape(w.Tool)
	case postEvent, failureEvent:
		event.Shape = model.ShapePost
		event.Post = &model.PostResult{Failed: w.Name == failureEvent}
	default:
		return model.Event{}, fmt.Errorf("%w: unsupported Claude event", model.ErrCannotJudge)
	}
	return event, nil
}

// preShape names the shape a pre event of a tool takes before any target
// is reconstructed, so enrollment tells a read from a write; a tool the
// adapter does not decode keeps no shape and the decoder refuses it.
func preShape(tool string) model.EventShape {
	switch tool {
	case writeTool, editTool, notebookTool:
		return model.ShapeWrite
	case readTool:
		return model.ShapeRead
	case bashTool, powerTool:
		return model.ShapeShell
	default:
		return 0
	}
}

// envelope decodes the envelope of an event in one pass and refuses the
// envelope of another agent and one without the event, the tool, the
// working directory or a tool input that is an object.
func envelope(data []byte) (wire, error) {
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return wire{}, fmt.Errorf("%w: Claude envelope: %w", model.ErrCannotJudge, err)
	}
	if w.Conversation != nil || w.Generation != nil || w.Roots != nil || w.Version != nil {
		return wire{}, fmt.Errorf("%w: envelope does not belong to claude", model.ErrCannotJudge)
	}
	if w.Name == "" || w.Tool == "" || w.Cwd == "" || len(w.Input) == 0 || w.Input[0] != '{' {
		return wire{}, fmt.Errorf("%w: incomplete Claude envelope", model.ErrCannotJudge)
	}
	return w, nil
}
