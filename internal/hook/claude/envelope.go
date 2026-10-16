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
}

// Header separates identity validation from enrolled target reconstruction.
func Header(data []byte) (model.Event, error) {
	w, err := envelope(data)
	if err != nil {
		return model.Event{}, err
	}
	event := model.Event{
		Agent: "claude", Tool: w.Tool, Cwd: w.Cwd,
		SessionID: w.Session, AgentID: w.Agent, ToolUseID: w.Use,
	}
	switch w.Name {
	case preEvent:
	case postEvent, failureEvent:
		event.Shape = model.ShapePost
		event.Post = &model.PostResult{Failed: w.Name == failureEvent}
	default:
		return model.Event{}, fmt.Errorf("%w: unsupported Claude event", model.ErrCannotJudge)
	}
	return event, nil
}

func envelope(data []byte) (wire, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return wire{}, fmt.Errorf("%w: Claude envelope: %w", model.ErrCannotJudge, err)
	}
	for _, key := range []string{
		"conversation_id", "generation_id", "workspace_roots", "cursor_version",
	} {
		if _, found := raw[key]; found {
			return wire{}, fmt.Errorf("%w: envelope does not belong to claude", model.ErrCannotJudge)
		}
	}
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return wire{}, fmt.Errorf("%w: Claude fields: %w", model.ErrCannotJudge, err)
	}
	var input map[string]json.RawMessage
	err := json.Unmarshal(w.Input, &input)
	if w.Name == "" || w.Tool == "" || w.Cwd == "" || err != nil || input == nil {
		return wire{}, fmt.Errorf("%w: incomplete Claude envelope", model.ErrCannotJudge)
	}
	return w, nil
}
