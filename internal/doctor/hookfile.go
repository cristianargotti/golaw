package doctor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/install"
)

// The markers of the block an installation writes into a file the person
// owns, as section 8.5 names them.
const (
	blockBegin = "<!-- golaw:begin -->"
	blockEnd   = "<!-- golaw:end -->"
)

// slot is one handler of a hooks file at the position its host keys it by:
// the group in the list of its event and the handler in the group (Cursor
// lists handlers without groups, so its group is the handler's index and
// its handler 0), the matcher that selects it and its exact JSON.
type slot struct {
	Group   int             `json:"group"`
	Handler int             `json:"handler"`
	Matcher string          `json:"matcher"`
	Raw     json.RawMessage `json:"raw"`
}

// groupedFile is a hooks file whose events list matcher groups, as Codex
// reads it; fields beside these are another tool's and are kept unread.
type groupedFile struct {
	Hooks map[string][]struct {
		Matcher string            `json:"matcher"`
		Hooks   []json.RawMessage `json:"hooks"`
	} `json:"hooks"`
}

// flatFile is a hooks file whose events list handlers, as Cursor reads it.
type flatFile struct {
	Hooks map[string][]json.RawMessage `json:"hooks"`
}

// handlerFields are the fields of a handler doctor reads.
type handlerFields struct {
	Matcher        string `json:"matcher"`
	Command        string `json:"command"`
	CommandWindows string `json:"commandWindows"`
	FailClosed     bool   `json:"failClosed"`
}

// readSlots reads the slots of every event of a hooks file.
func readSlots(path string, flat bool) (map[string][]slot, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the path comes from the installation receipt
	if err != nil {
		return nil, fmt.Errorf("read the hooks file: %w", err)
	}
	return parseSlots(data, flat)
}

// parseSlots lists the handlers of every event in file order.
func parseSlots(data []byte, flat bool) (map[string][]slot, error) {
	if flat {
		return flatSlots(data)
	}
	var file groupedFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("decode the hooks file: %w", err)
	}
	out := map[string][]slot{}
	for event, groups := range file.Hooks {
		for group, value := range groups {
			for handler, raw := range value.Hooks {
				out[event] = append(out[event], slot{Group: group, Handler: handler, Matcher: value.Matcher, Raw: raw})
			}
		}
	}
	return out, nil
}

// flatSlots lists the handlers of a file without groups, each its own
// group, its matcher read from the handler.
func flatSlots(data []byte) (map[string][]slot, error) {
	var file flatFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("decode the hooks file: %w", err)
	}
	out := map[string][]slot{}
	for event, handlers := range file.Hooks {
		for index, raw := range handlers {
			out[event] = append(out[event], slot{Group: index, Matcher: fieldsOf(raw).Matcher, Raw: raw})
		}
	}
	return out, nil
}

// fieldsOf decodes the fields doctor reads from a handler; a handler that
// does not decode has none.
func fieldsOf(raw json.RawMessage) handlerFields {
	var fields handlerFields
	if json.Unmarshal(raw, &fields) != nil {
		return handlerFields{}
	}
	return fields
}

// failsClosed reports whether the host blocks the tool when the handler
// fails: the Cursor field, which no Codex handler carries.
func failsClosed(raw json.RawMessage) bool {
	return fieldsOf(raw).FailClosed
}

// locate is the index of the slot that holds a definition, both compared
// as compact JSON so indentation is not drift, or -1.
func locate(slots []slot, definition json.RawMessage) int {
	want, ok := compact(definition)
	if !ok {
		return -1
	}
	return slices.IndexFunc(slots, func(s slot) bool {
		got, ok := compact(s.Raw)
		return ok && bytes.Equal(got, want)
	})
}

// compact is the JSON without insignificant space, false when it is not
// JSON at all.
func compact(raw json.RawMessage) ([]byte, bool) {
	var buffer bytes.Buffer
	if json.Compact(&buffer, raw) != nil {
		return nil, false
	}
	return buffer.Bytes(), true
}

// blockMatches reports whether the file holds the block with the hash the
// receipt recorded, from its begin marker through its end marker.
func blockMatches(block install.Block) bool {
	data, err := os.ReadFile(block.Path)
	if err != nil {
		return false
	}
	span, found := blockSpan(string(data))
	return found && install.Digest([]byte(span)) == block.SHA256
}

// blockSpan is the text from the begin marker through the end marker.
func blockSpan(content string) (string, bool) {
	begin := strings.Index(content, blockBegin)
	if begin < 0 {
		return "", false
	}
	end := strings.Index(content[begin:], blockEnd)
	if end < 0 {
		return "", false
	}
	return content[begin : begin+end+len(blockEnd)], true
}
