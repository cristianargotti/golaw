package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// shrunk is what the bounded reader keeps of an input over the bound: the
// raw value of each envelope field the hook reads, by the name the decoder
// matches it under, and the tool input.
type shrunk struct {
	fields map[string][]byte
	input  *shrunkInput
}

// shrunkInput is the tool input of an input over the bound: whether it is
// an object, the raw value of its command, the skeleton of that command
// read as a patch, and whether the command string was too long to keep.
type shrunkInput struct {
	object  bool
	command []byte
	patch   *skeleton
	elided  bool
}

// shrinkEnvelope reads one JSON object to the end of the input in bounded
// memory, checked whole as the standard decoder checks it, discarded
// fields and what follows the object included.
func shrinkEnvelope(input io.Reader) (shrunk, error) {
	s := scanner{r: bufio.NewReaderSize(input, readBuffer)}
	open, err := s.token()
	if err != nil {
		return shrunk{}, err
	}
	if open != '{' {
		return shrunk{}, malformed("an object")
	}
	out := shrunk{fields: map[string][]byte{}}
	if readErr := s.members(func(name string, first byte) error { return out.member(&s, name, first) }); readErr != nil {
		return shrunk{}, readErr
	}
	return out, s.end()
}

// envelopeField is the name of an envelope field the hook reads that a
// member name matches, as the standard decoder matches it, regardless of
// case; the four Cursor discriminators are read for their presence.
func envelopeField(name string) (string, bool) {
	for _, field := range []string{
		"hook_event_name", "tool_name", "cwd", "session_id", "turn_id", "agent_id", "tool_use_id",
		"tool_response", "conversation_id", "generation_id", "workspace_roots", "cursor_version",
	} {
		if strings.EqualFold(name, field) {
			return field, true
		}
	}
	return "", false
}

// member reads one member of the envelope: a field the hook reads is kept,
// the tool input is read for its command, any other value is checked and
// dropped, the last of a repeated field winning as with the decoder.
func (o *shrunk) member(s *scanner, name string, first byte) error {
	if strings.EqualFold(name, "tool_input") {
		return o.toolInput(s, first)
	}
	field, ok := envelopeField(name)
	if !ok {
		return s.value(first, 1)
	}
	raw, over, err := s.capture(first, fieldBound)
	if err != nil {
		return err
	}
	if over {
		if raw = stand(field); raw == nil {
			return fmt.Errorf("%w: the %s of an input over the bound is longer than the hook keeps", model.ErrCannotJudge, field)
		}
	}
	o.fields[field] = raw
	return nil
}

// stand is what the copy holds for a field too long to keep: a Cursor
// discriminator refuses the envelope whatever its value, and the response
// of a post event is read only for its presence; any other field cannot
// stand in for itself.
func stand(field string) []byte {
	switch field {
	case "tool_response":
		return []byte(`""`)
	case "conversation_id", "generation_id", "workspace_roots", "cursor_version":
		return []byte("null")
	default:
		return nil
	}
}

// toolInput reads the tool input: an object is read for its command, the
// one member the two canonical tools carry, and any other value is kept
// as a value that is not an object, which the envelope refuses.
func (o *shrunk) toolInput(s *scanner, first byte) error {
	o.input = &shrunkInput{object: first == '{'}
	if first != '{' {
		return s.value(first, 1)
	}
	return s.members(func(name string, b byte) error {
		if !strings.EqualFold(name, "command") {
			return s.value(b, 2)
		}
		return o.input.read(s, b)
	})
}

// read reads the command. A string streams, decoded, into the skeleton of
// a patch while its raw form is kept up to the bound of the hook; any other
// value is kept, or stands as an empty value of its type when it is too
// long, so the decoder refuses it in the words it uses within the bound.
func (in *shrunkInput) read(s *scanner, first byte) error {
	in.command, in.patch, in.elided = nil, nil, false
	if first != '"' {
		raw, over, err := s.capture(first, fieldBound)
		if over {
			raw = emptyOf(first)
		}
		in.command = raw
		return err
	}
	in.patch = &skeleton{}
	s.keep = &copied{limit: hook.MaxInputBytes}
	err := s.text(in.patch.write)
	in.command, in.elided = s.keep.raw, s.keep.over
	s.keep = nil
	return err
}

// document is the copy of the envelope the adapter parses as any event:
// the fields kept, and the tool input with its command, an empty string in
// place of a command too long to keep.
func (o *shrunk) document() []byte {
	doc := []byte{'{'}
	for _, field := range slices.Sorted(maps.Keys(o.fields)) {
		doc = appendMember(doc, field, o.fields[field])
	}
	if o.input != nil {
		doc = appendMember(doc, "tool_input", o.input.document())
	}
	if doc[len(doc)-1] == ',' {
		doc[len(doc)-1] = '}'
		return doc
	}
	return append(doc, '}')
}

func (in *shrunkInput) document() []byte {
	switch {
	case !in.object:
		return []byte("[]")
	case in.elided:
		return []byte(`{"command":""}`)
	case in.command == nil:
		return []byte("{}")
	default:
		return append(append([]byte(`{"command":`), in.command...), '}')
	}
}

// capture reads a value whose first byte was read and answers its copy up
// to limit, and whether it was longer.
func (s *scanner) capture(first byte, limit int) ([]byte, bool, error) {
	s.keep = &copied{limit: limit}
	err := s.value(first, 1)
	kept := s.keep
	s.keep = nil
	return kept.raw, kept.over, err
}

// label reads a member name whose first byte was read: into the copy when
// one is kept, else decoded when it is short enough to be a field the hook
// reads, and empty otherwise.
func (s *scanner) label(first byte) (string, error) {
	if first != '"' {
		return "", malformed("a member name")
	}
	if s.keep != nil {
		return "", s.text(nil)
	}
	s.keep = &copied{limit: nameBound}
	err := s.text(nil)
	kept := s.keep
	s.keep = nil
	var name string
	if err == nil && !kept.over && json.Unmarshal(kept.raw, &name) != nil {
		name = ""
	}
	return name, err
}
