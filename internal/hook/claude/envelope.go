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
		Agent: "claude", Tool: w.Tool, Cwd: w.Cwd, Target: target(w),
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

// target is the path a write or read names, the location its enrollment
// is resolved from (A3): the repository that holds the file governs it,
// wherever the session runs. A shell tool names none and is judged where
// it runs; a path that is not a string is left to the decoder to refuse.
func target(w wire) string {
	key := "file_path"
	switch w.Tool {
	case writeTool, editTool, readTool:
	case notebookTool:
		key = "notebook_path"
	default:
		return ""
	}
	var input map[string]json.RawMessage
	var path string
	if json.Unmarshal(w.Input, &input) != nil || json.Unmarshal(input[key], &path) != nil {
		return ""
	}
	return path
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
