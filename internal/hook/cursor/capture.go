package cursor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// writeEnforced is the one switch of D8: false while the Write and Delete
// payloads stay undocumented, so they are captured and allowed (A4); true
// selects a typed decoder a later version supplies, and until it exists
// the true branch cannot judge, so flipping the switch alone never invents
// enforcement.
const writeEnforced = false

// captureRequest is the preview request of a preToolUse Write or Delete:
// the key names of its input recorded for doctor before the answer, the
// baseline of A15 kept for the post event, and the event allowed by no
// rule, with no target and no candidate so enrollment reads the working
// directory and the engine never runs.
func captureRequest(input payload, event model.Event) (hook.Request, error) {
	keys, err := captureKeys(input["tool_input"])
	if err != nil {
		return hook.Request{}, err
	}
	return hook.Request{
		Event: model.Event{
			Shape:     model.ShapeWrite,
			Agent:     agentID,
			Tool:      event.Tool,
			Cwd:       event.Cwd,
			SessionID: event.SessionID,
			AgentID:   event.AgentID,
			ToolUseID: event.ToolUseID,
		},
		Observe: true,
		Capture: &hook.Capture{Event: preEvent, Tool: event.Tool, Keys: keys},
	}, nil
}

// captureKeys are the key names of a tool input, sorted and unique, a
// nested object's keys joined to their parent with a dot and the objects
// of an array visited without their positions, because the record names
// keys, never values or positions. An input that is no object is named by
// its JSON type alone: the payload is undocumented and Cursor stringifies
// other fields, so an unknown shape is captured and allowed rather than
// denied (A4).
func captureKeys(input json.RawMessage) ([]string, error) {
	trimmed := bytes.TrimSpace(input)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return []string{jsonType(trimmed)}, nil
	}
	var object map[string]any
	if err := json.Unmarshal(trimmed, &object); err != nil {
		return nil, inputError(err)
	}
	names := map[string]struct{}{}
	collectKeys("", object, names)
	return slices.Sorted(maps.Keys(names)), nil
}

// inputError is the reason of a tool input the decoder could not read:
// cannot judge, never a verdict the policy did not give.
func inputError(err error) error {
	return fmt.Errorf("%w: cursor tool_input: %w", model.ErrCannotJudge, err)
}

// collectKeys adds the keys of an object under prefix and descends into
// what each value holds.
func collectKeys(prefix string, object map[string]any, names map[string]struct{}) {
	for name, value := range object {
		full := prefix + name
		names[full] = struct{}{}
		collectNested(full+".", value, names)
	}
}

// collectNested descends into a value that may hold objects: an object by
// its keys under prefix, an array by the objects it holds, and a scalar
// adds nothing.
func collectNested(prefix string, value any, names map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		collectKeys(prefix, typed, names)
	case []any:
		for _, item := range typed {
			collectNested(prefix, item, names)
		}
	}
}

// jsonType names the JSON type of a value by its first byte, exact because
// the envelope already validated the document: the record says what shape
// arrived without reading it.
func jsonType(value []byte) string {
	switch {
	case len(value) == 0:
		return "<absent>"
	case value[0] == '[':
		return "<array>"
	case value[0] == '"':
		return "<string>"
	case value[0] == 'n':
		return "<null>"
	case value[0] == 't', value[0] == 'f':
		return "<boolean>"
	default:
		return "<number>"
	}
}
