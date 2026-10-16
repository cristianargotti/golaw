package claude

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/cristianargotti/golaw/internal/model"
)

// maxField bounds a string the skim keeps: a header field, a key or a path.
const maxField = 64 << 10

// skimmed is what a skim of an event keeps: the string fields of its
// header, the path of its tool input and whether a field of another agent
// shows.
type skimmed struct {
	fields  map[string]string
	path    string
	foreign bool
}

// skimmer reads JSON a byte at a time from a buffered reader.
type skimmer struct{ r *bufio.Reader }

// errShape is the error of an input that is not one JSON object.
func errShape() error {
	return fmt.Errorf("%w: the input is not one JSON object", model.ErrCannotJudge)
}

// skim reads one JSON object to its end and to the end of the input in
// bounded memory: the string value of each header field and of the path of
// the tool input are kept, and every other value, the content of a write
// of any size included, is read past without being held.
func skim(r io.Reader) (skimmed, error) {
	s := skimmer{r: bufio.NewReader(r)}
	out := skimmed{fields: map[string]string{}}
	err := s.object(func(key string) error {
		out.foreign = out.foreign || foreignKey(key)
		switch {
		case key == "tool_input":
			return s.toolInput(&out)
		case headerKey(key):
			value, kept, err := s.keptString()
			if kept {
				out.fields[key] = value
			}
			return err
		default:
			return s.skipValue()
		}
	})
	if err == nil {
		err = s.end()
	}
	return out, err
}

// toolInput reads the tool input, keeping the path a write or a notebook
// edit names.
func (s *skimmer) toolInput(out *skimmed) error {
	b, err := s.next()
	if err != nil || b != '{' {
		if err == nil {
			err = s.skipFrom(b)
		}
		return err
	}
	return s.members(func(key string) error {
		if key != "file_path" && key != "notebook_path" {
			return s.skipValue()
		}
		value, kept, err := s.keptString()
		if kept {
			out.path = value
		}
		return err
	})
}

// next answers the next byte that is not white space.
func (s *skimmer) next() (byte, error) {
	for {
		b, err := s.r.ReadByte()
		if err != nil || b != ' ' && b != '\t' && b != '\n' && b != '\r' {
			return b, err //nolint:wrapcheck // the skim names the shape of its input
		}
	}
}

// object reads an object, handing the key of each member to member, which
// reads its value.
func (s *skimmer) object(member func(key string) error) error {
	b, err := s.next()
	if err != nil {
		return err
	}
	if b != '{' {
		return errShape()
	}
	return s.members(member)
}

// members reads the members of an object whose opening brace was read.
func (s *skimmer) members(member func(key string) error) error {
	b, err := s.next()
	if err != nil || b == '}' {
		return err
	}
	for {
		if memberErr := s.member(b, member); memberErr != nil {
			return memberErr
		}
		if b, err = s.next(); err != nil || b == '}' {
			return err
		}
		if b != ',' {
			return errShape()
		}
		if b, err = s.next(); err != nil {
			return err
		}
	}
}

// member reads one member whose first byte was read: its key, its colon
// and, through member, its value.
func (s *skimmer) member(b byte, member func(key string) error) error {
	if b != '"' {
		return errShape()
	}
	key, _, err := s.str(true)
	if err != nil {
		return err
	}
	if b, err = s.next(); err != nil {
		return err
	}
	if b != ':' {
		return errShape()
	}
	return member(key)
}

// keptString reads a value, kept when it is a string that fits.
func (s *skimmer) keptString() (string, bool, error) {
	b, err := s.next()
	if err != nil {
		return "", false, err
	}
	if b != '"' {
		return "", false, s.skipFrom(b)
	}
	return s.str(true)
}

// str reads a string whose opening quote was read, keeping and decoding it
// when keep is true and its raw bytes fit in maxField.
func (s *skimmer) str(keep bool) (string, bool, error) {
	var raw []byte
	for {
		b, err := s.r.ReadByte()
		if err != nil {
			return "", false, err //nolint:wrapcheck // the skim names the shape of its input
		}
		if b == '"' {
			return decoded(raw, keep)
		}
		if b == '\\' {
			escaped, err := s.r.ReadByte()
			if err != nil {
				return "", false, err //nolint:wrapcheck // the skim names the shape of its input
			}
			raw = append(raw, b, escaped)
		} else {
			raw = append(raw, b)
		}
		if !keep || len(raw) > maxField {
			keep, raw = false, raw[:0]
		}
	}
}

// decoded is the value of the raw bytes of a string, when kept.
func decoded(raw []byte, keep bool) (string, bool, error) {
	if !keep {
		return "", false, nil
	}
	var value string
	if err := json.Unmarshal(append(append([]byte{'"'}, raw...), '"'), &value); err != nil {
		return "", false, errShape()
	}
	return value, true, nil
}

// skipValue reads past one value without holding it.
func (s *skimmer) skipValue() error {
	b, err := s.next()
	if err != nil {
		return err
	}
	return s.skipFrom(b)
}

// skipFrom reads past the value whose first byte was read.
func (s *skimmer) skipFrom(b byte) error {
	switch {
	case b == '"':
		_, _, err := s.str(false)
		return err
	case b == '{' || b == '[':
		return s.skipNested()
	case scalar(b):
		for {
			next, err := s.r.ReadByte()
			if err != nil || !scalar(next) {
				if err == nil {
					err = s.r.UnreadByte()
				}
				return err //nolint:wrapcheck // the skim names the shape of its input
			}
		}
	default:
		return errShape()
	}
}

// skipNested reads past an object or an array whose opening bracket was
// read, the strings inside it included.
func (s *skimmer) skipNested() error {
	for depth := 1; depth > 0; {
		b, err := s.r.ReadByte()
		if err != nil {
			return err //nolint:wrapcheck // the skim names the shape of its input
		}
		switch b {
		case '"':
			if _, _, err := s.str(false); err != nil {
				return err
			}
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		}
	}
	return nil
}

// scalar reports a byte a number, true, false or null may hold.
func scalar(b byte) bool {
	return b == '-' || b == '+' || b == '.' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// end requires nothing but white space after the object.
func (s *skimmer) end() error {
	if _, err := s.next(); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errShape()
		}
		return err
	}
	return nil
}
