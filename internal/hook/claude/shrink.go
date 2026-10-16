package claude

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// The bounds of a shrink: a string under a key that names the event or a
// file is kept up to keptField bytes and any other up to keptText, and a
// longer one is elided; the input is read one buffer at a time.
const (
	keptField    = 64 << 10
	keptText     = 4 << 10
	shrinkBuffer = 64 << 10
)

// elided is the string a shrink writes in place of one it does not keep: a
// NUL, which no field the hook reads may hold, then words for a reader.
const elided = `"\u0000elided by golaw"`

// The states of a shrink: what the next byte that is not white space
// begins or ends.
const (
	wantValue = iota
	wantValueOrEnd
	wantKey
	wantKeyOrEnd
	wantColon
	wantMore
)

// level is one open container of a shrink, an object or an array, with the
// key its strings are kept by when it is an array: the key it is the value
// of.
type level struct {
	kind byte
	key  string
}

// shrinker copies one JSON value from a buffered reader.
type shrinker struct {
	r    *bufio.Reader
	out  []byte
	open []level
	key  string // the key of the member whose value is read next
}

// shrink reads one JSON value to the end of the input in bounded memory and
// answers a copy of it in which every string longer than it keeps is
// elided. The grammar of the whole input is checked as the decoder checks
// it, so the copy decodes as the input would but for the strings it elides,
// and a copy over the bound of an event is refused.
func shrink(r io.Reader) ([]byte, error) {
	s := shrinker{r: bufio.NewReaderSize(r, shrinkBuffer)}
	for state := wantValue; ; {
		b, err := s.next()
		if err != nil {
			return nil, ended(err)
		}
		if state, err = s.step(state, b); err != nil {
			return nil, err
		}
		if len(s.out) > model.MaxReadBytes {
			return nil, fmt.Errorf("%w: an input over the bound holds more than the hook reads outside its long strings", model.ErrCannotJudge)
		}
		if state == wantMore && len(s.open) == 0 {
			return s.out, s.end()
		}
	}
}

// step reads what b begins in a state and answers the next state.
func (s *shrinker) step(state int, b byte) (int, error) {
	switch state {
	case wantValueOrEnd:
		if b == ']' {
			return s.close(b), nil
		}
		return s.value(b)
	case wantKeyOrEnd:
		if b == '}' {
			return s.close(b), nil
		}
		return s.member(b)
	case wantKey:
		return s.member(b)
	case wantColon:
		if b != ':' {
			return 0, invalid("a colon after a key")
		}
		s.out = append(s.out, b)
		return wantValue, nil
	case wantMore:
		return s.more(b)
	default:
		return s.value(b)
	}
}

// value reads a value whose first byte was read.
func (s *shrinker) value(b byte) (int, error) {
	key := s.key
	if n := len(s.open); n > 0 && s.open[n-1].kind == '[' {
		key = s.open[n-1].key
	}
	switch {
	case b == '{' || b == '[':
		s.open = append(s.open, level{kind: b, key: key})
		s.out = append(s.out, b)
		if b == '{' {
			return wantKeyOrEnd, nil
		}
		return wantValueOrEnd, nil
	case b == '"':
		return wantMore, s.str(kept(key))
	case b == '-' || b >= '0' && b <= '9':
		return wantMore, s.number(b)
	case b == 't' || b == 'f' || b == 'n':
		return wantMore, s.literal(b)
	default:
		return 0, invalid("the beginning of a value")
	}
}

// member reads the key of an object member whose first byte was read.
func (s *shrinker) member(b byte) (int, error) {
	if b != '"' {
		return 0, invalid("a key")
	}
	start := len(s.out)
	if err := s.str(keptField); err != nil {
		return 0, err
	}
	s.key = ""
	if raw := s.out[start:]; string(raw) != elided {
		if err := json.Unmarshal(raw, &s.key); err != nil {
			return 0, invalid("a key")
		}
	}
	return wantColon, nil
}

// more reads what follows a value inside a container: a comma, or the end
// of that container.
func (s *shrinker) more(b byte) (int, error) {
	kind := s.open[len(s.open)-1].kind
	switch {
	case b == ',' && kind == '{':
		s.out = append(s.out, b)
		return wantKey, nil
	case b == ',':
		s.out = append(s.out, b)
		return wantValue, nil
	case b == '}' && kind == '{', b == ']' && kind == '[':
		return s.close(b), nil
	default:
		return 0, invalid("a comma or the end of a container")
	}
}

// close ends the innermost container.
func (s *shrinker) close(b byte) int {
	s.open = s.open[:len(s.open)-1]
	s.out = append(s.out, b)
	return wantMore
}

// kept is the size up to which a string under a key is kept: the event,
// its identities and the files it names are fields the hook reads.
func kept(key string) int {
	switch strings.ToLower(key) {
	case "hook_event_name", "tool_name", "cwd", "session_id", "agent_id", "tool_use_id",
		"file_path", "notebook_path", "command", "changedfiles", "filepath":
		return keptField
	default:
		return keptText
	}
}

// next answers the next byte that is not white space.
func (s *shrinker) next() (byte, error) {
	for {
		b, err := s.r.ReadByte()
		if err != nil || b != ' ' && b != '\t' && b != '\n' && b != '\r' {
			return b, err //nolint:wrapcheck // ended names the end of the input
		}
	}
}

// end requires nothing but white space after the value.
func (s *shrinker) end() error {
	if _, err := s.next(); !errors.Is(err, io.EOF) {
		if err != nil {
			return ended(err)
		}
		return invalid("the end of the input after its value")
	}
	return nil
}

// invalid is the refusal of an input that breaks the grammar.
func invalid(where string) error {
	return fmt.Errorf("%w: the input is not valid JSON at %s", model.ErrCannotJudge, where)
}

// ended is the refusal of an input that ends inside a value or cannot be
// read.
func ended(err error) error {
	if errors.Is(err, io.EOF) {
		return invalid("its end, inside a value")
	}
	return fmt.Errorf("%w: read input: %w", model.ErrCannotJudge, err)
}
