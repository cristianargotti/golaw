package claude

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

// Oversized reads the header of an event whose input is over the bound of
// the hook from the prefix the hook read: the event name, the tool, the
// working directory, the identities and the path a write names, each as
// far as the prefix holds it, the host writing the tool input after the
// rest of the header. A prefix without that header, or with a field of
// another agent, cannot be judged.
func Oversized(prefix []byte) (model.Event, error) {
	fields, path, foreign := scanPrefix(prefix)
	w := wire{
		Name: fields["hook_event_name"], Tool: fields["tool_name"], Cwd: fields["cwd"],
		Session: fields["session_id"], Agent: fields["agent_id"], Use: fields["tool_use_id"],
	}
	if foreign || w.Name == "" || w.Tool == "" || w.Cwd == "" {
		return model.Event{}, fmt.Errorf("%w: the header of an input over the bound is incomplete", model.ErrCannotJudge)
	}
	event := model.Event{
		Agent: "claude", Tool: w.Tool, Cwd: w.Cwd, Target: path,
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

// scanPrefix walks the top level of the envelope up to its tool input,
// which the host writes after the rest of the header: the string fields of
// the header, the path of the tool input and whether a field of another
// agent shows. It stops at the tool input, so the content it carries is
// never read.
func scanPrefix(prefix []byte) (map[string]string, string, bool) {
	decoder := json.NewDecoder(bytes.NewReader(prefix))
	fields := map[string]string{}
	foreign := false
	if open, err := decoder.Token(); err != nil || open != json.Delim('{') {
		return fields, "", false
	}
	for decoder.More() {
		key, err := decoder.Token()
		name, isKey := key.(string)
		if err != nil || !isKey {
			break
		}
		foreign = foreign || foreignKey(name)
		if name == "tool_input" {
			return fields, scanInput(decoder), foreign
		}
		if keep(decoder, name, fields) != nil {
			break
		}
	}
	return fields, "", foreign
}

// keep reads the value of a field and keeps it when it is a string of the
// header.
func keep(decoder *json.Decoder, name string, fields map[string]string) error {
	var value json.RawMessage
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("prefix value: %w", err)
	}
	var text string
	if headerKey(name) && json.Unmarshal(value, &text) == nil {
		fields[name] = text
	}
	return nil
}

// headerKey reports a field of the envelope Oversized keeps.
func headerKey(name string) bool {
	switch name {
	case "hook_event_name", "tool_name", "cwd", "session_id", "agent_id", "tool_use_id":
		return true
	default:
		return false
	}
}

// scanInput walks the tool input up to the path a write names, empty when
// the prefix ends before it, as it does when the path comes after the
// content.
func scanInput(decoder *json.Decoder) string {
	if open, err := decoder.Token(); err != nil || open != json.Delim('{') {
		return ""
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return ""
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return ""
		}
		var text string
		name, isKey := key.(string)
		if isKey && (name == "file_path" || name == "notebook_path") && json.Unmarshal(value, &text) == nil {
			return text
		}
	}
	return ""
}

// foreignKey reports a field of the envelope of another agent.
func foreignKey(name string) bool {
	switch name {
	case "conversation_id", "generation_id", "workspace_roots", "cursor_version":
		return true
	default:
		return false
	}
}
