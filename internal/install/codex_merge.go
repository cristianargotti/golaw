package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
)

const (
	codexDescription = "description"
	codexHooksKey    = "hooks"
	codexMatcherKey  = "matcher"
)

// codexHooksFile is a Codex hooks file held for a merge: its top-level
// keys in file order with their raw values, and under hooks each event in
// file order with its groups raw, so another tool's groups, handlers and
// fields are written back in their order and with their values as read.
type codexHooksFile struct {
	keys   []string
	values map[string]json.RawMessage
	events []string
	groups map[string][]json.RawMessage
}

// codexGroup is the part of a matcher group golaw reads: its matcher, nil
// when absent, and its handlers raw.
type codexGroup struct {
	matcher *string
	hooks   []json.RawMessage
}

// codexPosition is one handler at the position Codex keys its trust by.
type codexPosition struct {
	event          string
	group, handler int
}

// parseCodexHooks reads a hooks file the way Codex accepts it: only
// description and hooks at the top level, since Codex drops the whole
// file for any other key, every event a list of groups and every group an
// object. An empty file holds nothing to keep.
func parseCodexHooks(data []byte) (codexHooksFile, error) {
	file := codexHooksFile{values: map[string]json.RawMessage{}, groups: map[string][]json.RawMessage{}}
	if len(bytes.TrimSpace(data)) == 0 {
		return file, nil
	}
	keys, values, err := rawObjectFields(data)
	if err != nil {
		return codexHooksFile{}, err
	}
	file.keys, file.values = keys, values
	for _, key := range keys {
		switch key {
		case codexDescription:
			var description *string
			if json.Unmarshal(values[key], &description) != nil {
				err = fmt.Errorf("the description is no string: %w", ErrSettings)
			}
		case codexHooksKey:
			err = file.readEvents(values[key])
		default:
			err = fmt.Errorf("top-level key %q makes Codex refuse the whole file: %w", key, ErrSettings)
		}
		if err != nil {
			return codexHooksFile{}, err
		}
	}
	return file, nil
}

func (f *codexHooksFile) readEvents(raw json.RawMessage) error {
	events, values, err := rawObjectFields(raw)
	if err != nil {
		return err
	}
	for _, event := range events {
		var groups []json.RawMessage
		if !bytes.HasPrefix(values[event], []byte("[")) || json.Unmarshal(values[event], &groups) != nil {
			return fmt.Errorf("event %s is no list of matcher groups: %w", event, ErrSettings)
		}
		for index, group := range groups {
			if _, err := decodeCodexGroup(group); err != nil {
				return fmt.Errorf("event %s group %d: %w", event, index, err)
			}
		}
		f.events, f.groups[event] = append(f.events, event), groups
	}
	return nil
}

// decodeCodexGroup reads the matcher and the handlers of a group by their
// exact names, as Codex reads them.
func decodeCodexGroup(raw json.RawMessage) (codexGroup, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return codexGroup{}, fmt.Errorf("matcher group is no object: %w", ErrSettings)
	}
	var group codexGroup
	if value, found := fields[codexMatcherKey]; found && json.Unmarshal(value, &group.matcher) != nil {
		return codexGroup{}, fmt.Errorf("matcher is no string: %w", ErrSettings)
	}
	if value, found := fields[codexHooksKey]; found && json.Unmarshal(value, &group.hooks) != nil {
		return codexGroup{}, fmt.Errorf("handlers are no list: %w", ErrSettings)
	}
	return group, nil
}

// locate compares handlers as canonical JSON, so a file an editor or
// another tool rewrote with its keys in another order still shows golaw's
// handler, and answers every position, so a copy is never missed.
func (f *codexHooksFile) locate(event, matcher string, definition json.RawMessage) []codexPosition {
	want, ok := CanonicalJSON(definition)
	if !ok {
		return nil
	}
	var found []codexPosition
	for group, raw := range f.groups[event] {
		decoded, err := decodeCodexGroup(raw)
		if err != nil || decoded.matcher == nil || *decoded.matcher != matcher {
			continue
		}
		for handler, candidate := range decoded.hooks {
			if got, ok := CanonicalJSON(candidate); ok && bytes.Equal(got, want) {
				found = append(found, codexPosition{event: event, group: group, handler: handler})
			}
		}
	}
	return found
}

// foreign lists every handler of the file outside owned, in file order:
// a merge never reorders them, so two listings pair up index by index.
func (f *codexHooksFile) foreign(owned []codexPosition) []codexPosition {
	var out []codexPosition
	for _, event := range f.events {
		for group, raw := range f.groups[event] {
			decoded, err := decodeCodexGroup(raw)
			if err != nil {
				continue
			}
			for handler := range decoded.hooks {
				position := codexPosition{event: event, group: group, handler: handler}
				if !slices.Contains(owned, position) {
					out = append(out, position)
				}
			}
		}
	}
	return out
}

// replaceHandler writes definition over the handler at position and keeps
// every other field and handler of its group.
func (f *codexHooksFile) replaceHandler(position codexPosition, definition json.RawMessage) error {
	raw := f.groups[position.event][position.group]
	keys, values, err := rawObjectFields(raw)
	if err != nil {
		return err
	}
	var handlers []json.RawMessage
	if json.Unmarshal(values[codexHooksKey], &handlers) != nil || position.handler >= len(handlers) {
		return fmt.Errorf("handlers of %s group %d: %w", position.event, position.group, ErrSettings)
	}
	handlers[position.handler] = definition
	values[codexHooksKey] = rawArray(handlers)
	group, err := rawObject(keys, values)
	if err != nil {
		return err
	}
	f.groups[position.event][position.group] = group
	return nil
}

// appendGroup adds group after every group of event, where it shifts no
// other handler's trust key.
func (f *codexHooksFile) appendGroup(event string, group json.RawMessage) codexPosition {
	if _, known := f.groups[event]; !known {
		f.events = append(f.events, event)
	}
	f.groups[event] = append(f.groups[event], group)
	return codexPosition{event: event, group: len(f.groups[event]) - 1}
}

// encode keeps the raw values as read and the keys in their order, so a
// second install writes the same bytes and prints no diff.
func (f *codexHooksFile) encode() ([]byte, error) {
	keys, values := slices.Clone(f.keys), maps.Clone(f.values)
	if len(f.events) > 0 && !slices.Contains(keys, codexHooksKey) {
		keys = append(keys, codexHooksKey)
	}
	if slices.Contains(keys, codexHooksKey) {
		events := make(map[string]json.RawMessage, len(f.events))
		for _, event := range f.events {
			events[event] = rawArray(f.groups[event])
		}
		hooks, err := rawObject(f.events, events)
		if err != nil {
			return nil, err
		}
		values[codexHooksKey] = hooks
	}
	document, err := rawObject(keys, values)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, document, "", "  "); err != nil {
		return nil, fmt.Errorf("encode the codex hooks: %w", err)
	}
	return append(out.Bytes(), '\n'), nil
}
