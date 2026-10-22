package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
)

const (
	cursorVersionKey = "version"
	cursorHooksKey   = "hooks"
	cursorCommandKey = "command"
)

// cursorHooksFile is a Cursor hooks file held for a merge: its top-level
// keys in file order with their raw values, and under hooks each event in
// file order with its handlers raw, so another tool's handlers and fields
// are written back in their order and with their values as read.
type cursorHooksFile struct {
	keys     []string
	values   map[string]json.RawMessage
	events   []string
	handlers map[string][]json.RawMessage
}

// cursorCompact is a handler without insignificant space, the form a
// receipt records and doctor compares, so indentation is never drift.
func cursorCompact(data json.RawMessage) ([]byte, error) {
	var out bytes.Buffer
	if err := json.Compact(&out, data); err != nil {
		return nil, fmt.Errorf("compact the cursor handler: %w", err)
	}
	return out.Bytes(), nil
}

// cursorMerge merges the handlers golaw wants into a hooks file: a handler
// a previous receipt records is replaced where it stands, a new one is
// appended after every handler of its event, and nothing else moves, so
// the order the person gave the other tools' handlers stays.
func cursorMerge(current []byte, wanted map[string][]json.RawMessage, previous []Entry) ([]byte, []Entry, error) {
	file, err := parseCursorHooks(current)
	if err != nil {
		return nil, nil, err
	}
	var entries []Entry
	for _, event := range cursorEvents() {
		for _, definition := range wanted[event] {
			entry, placeErr := file.place(event, definition, previous)
			if placeErr != nil {
				return nil, nil, placeErr
			}
			entries = append(entries, entry)
		}
	}
	merged, err := file.encode()
	if err != nil {
		return nil, nil, err
	}
	return merged, entries, nil
}

// parseCursorHooks reads a version 1 hooks file the way Cursor reads it:
// the version, when present, is 1, hooks is an object, every event a list
// and every handler an object. Another version is refused rather than
// rewritten, since its meaning is not the one golaw merges into; an
// empty file holds nothing to keep.
func parseCursorHooks(data []byte) (cursorHooksFile, error) {
	file := cursorHooksFile{values: map[string]json.RawMessage{}, handlers: map[string][]json.RawMessage{}}
	if len(bytes.TrimSpace(data)) == 0 {
		return file, nil
	}
	keys, values, err := rawObjectFields(data)
	if err != nil {
		return cursorHooksFile{}, fmt.Errorf("%w: %w", ErrHooks, err)
	}
	file.keys, file.values = keys, values
	if raw, found := values[cursorVersionKey]; found {
		var version float64
		if json.Unmarshal(raw, &version) != nil || version != 1 {
			return cursorHooksFile{}, fmt.Errorf("%w: version %s is not 1, the version golaw merges into", ErrHooks, raw)
		}
	}
	if raw, found := values[cursorHooksKey]; found {
		if err := file.readEvents(raw); err != nil {
			return cursorHooksFile{}, err
		}
	}
	return file, nil
}

func (f *cursorHooksFile) readEvents(raw json.RawMessage) error {
	events, values, err := rawObjectFields(raw)
	if err != nil {
		return fmt.Errorf("%w: hooks: %w", ErrHooks, err)
	}
	for _, event := range events {
		var handlers []json.RawMessage
		if !bytes.HasPrefix(values[event], []byte("[")) || json.Unmarshal(values[event], &handlers) != nil {
			return fmt.Errorf("%w: event %s is no list of handlers", ErrHooks, event)
		}
		for index, handler := range handlers {
			var fields map[string]json.RawMessage
			if json.Unmarshal(handler, &fields) != nil || fields == nil {
				return fmt.Errorf("%w: handler %d of %s is no object", ErrHooks, index, event)
			}
		}
		f.events, f.handlers[event] = append(f.events, event), handlers
	}
	return nil
}

// place puts golaw's handler of event into the file and answers its entry.
func (f *cursorHooksFile) place(event string, definition json.RawMessage, previous []Entry) (Entry, error) {
	entry := Entry{Event: event, Matcher: cursorMatcherOf(definition), Definition: definition}
	index, located, err := f.owned(event, previous)
	if err != nil {
		return Entry{}, err
	}
	if located {
		f.handlers[event][index] = definition
		entry.Group = index
		return entry, nil
	}
	if _, known := f.handlers[event]; !known {
		f.events = append(f.events, event)
	}
	f.handlers[event] = append(f.handlers[event], definition)
	entry.Group = len(f.handlers[event]) - 1
	return entry, nil
}

// encode keeps the raw values as read and the keys in their order, with
// the version written first when the file had none, so a second install
// writes the same bytes and prints no diff.
func (f *cursorHooksFile) encode() ([]byte, error) {
	keys, values := slices.Clone(f.keys), maps.Clone(f.values)
	if !slices.Contains(keys, cursorVersionKey) {
		keys, values[cursorVersionKey] = append([]string{cursorVersionKey}, keys...), json.RawMessage("1")
	}
	if !slices.Contains(keys, cursorHooksKey) {
		keys = append(keys, cursorHooksKey)
	}
	events := make(map[string]json.RawMessage, len(f.events))
	for _, event := range f.events {
		events[event] = rawArray(f.handlers[event])
	}
	hooks, err := rawObject(f.events, events)
	if err != nil {
		return nil, err
	}
	values[cursorHooksKey] = hooks
	document, err := rawObject(keys, values)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, document, "", "  "); err != nil {
		return nil, fmt.Errorf("encode the cursor hooks: %w", err)
	}
	return append(out.Bytes(), '\n'), nil
}
