package codex

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/cristianargotti/golaw/internal/model"
)

// The two events the installed matcher selects and the two canonical tool
// names Codex writes on stdin for them; Write and Edit are matcher aliases
// of apply_patch that never reach stdin (survey section 2.1).
const (
	preEvent  = "PreToolUse"
	postEvent = "PostToolUse"
	bashTool  = "Bash"
	patchTool = "apply_patch"
)

// wire is the envelope of a Codex event (D16). The four fields after the
// tool fields belong only to the envelope of Cursor: any of them, null
// included, refuses the event.
type wire struct {
	Name     string          `json:"hook_event_name"`
	Tool     string          `json:"tool_name"`
	Cwd      string          `json:"cwd"`
	Session  string          `json:"session_id"`
	Turn     string          `json:"turn_id"`
	Agent    string          `json:"agent_id"`
	Use      string          `json:"tool_use_id"`
	Input    json.RawMessage `json:"tool_input"`
	Response json.RawMessage `json:"tool_response"`

	Conversation json.RawMessage `json:"conversation_id"`
	Generation   json.RawMessage `json:"generation_id"`
	Roots        json.RawMessage `json:"workspace_roots"`
	Version      json.RawMessage `json:"cursor_version"`
}

// envelope decodes the one JSON object of an event and refuses the
// envelope of another agent, one followed by another value, one without a
// field Codex always writes and one whose tool input is not an object; any
// other field is tolerated, so a field a later Codex adds never denies
// every call (risk 2).
func envelope(data []byte) (wire, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var w wire
	if err := decoder.Decode(&w); err != nil {
		return wire{}, fmt.Errorf("%w: Codex envelope: %w", model.ErrCannotJudge, err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return wire{}, fmt.Errorf("%w: Codex envelope: a value follows the event", model.ErrCannotJudge)
	}
	if w.Conversation != nil || w.Generation != nil || w.Roots != nil || w.Version != nil {
		return wire{}, fmt.Errorf("%w: envelope does not belong to codex", model.ErrCannotJudge)
	}
	if missing := w.missing(); missing != "" {
		return wire{}, fmt.Errorf("%w: incomplete Codex envelope: %s", model.ErrCannotJudge, missing)
	}
	return w, nil
}

// missing names the first field Codex always writes that the envelope
// lacks, or the tool input that is not an object, so the reason says what
// to fix; empty when the envelope is complete.
func (w wire) missing() string {
	for _, field := range []struct{ name, value string }{
		{"hook_event_name", w.Name},
		{"tool_name", w.Tool},
		{"cwd", w.Cwd},
		{"session_id", w.Session},
		{"turn_id", w.Turn},
		{"tool_use_id", w.Use},
	} {
		if field.value == "" {
			return field.name + " is absent"
		}
	}
	if len(w.Input) == 0 || w.Input[0] != '{' {
		return "tool_input is not an object"
	}
	return ""
}

// header is the event of an envelope before any target is reconstructed:
// its identities, the host's agent_id kept as given and turn_id read as an
// envelope discriminator only, and the shape its event and tool give it.
func header(w wire) (model.Event, error) {
	event := model.Event{
		Agent: "codex", Tool: w.Tool, Cwd: w.Cwd,
		SessionID: w.Session, AgentID: w.Agent, ToolUseID: w.Use,
	}
	switch w.Name {
	case preEvent:
		event.Shape = preShape(w.Tool)
	case postEvent:
		event.Shape = model.ShapePost
		event.Post = &model.PostResult{}
	default:
		return model.Event{}, fmt.Errorf("%w: unsupported Codex event %s", model.ErrCannotJudge, w.Name)
	}
	return event, nil
}

// preShape names the shape a pre event of a tool takes before any target
// is reconstructed, so enrollment tells a write from a command; a tool the
// adapter does not decode keeps no shape and the decoder refuses it once
// the event is enrolled.
func preShape(tool string) model.EventShape {
	switch tool {
	case patchTool:
		return model.ShapeWrite
	case bashTool:
		return model.ShapeShell
	default:
		return 0
	}
}
