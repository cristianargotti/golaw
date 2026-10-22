package codex

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// The bounds of the reader of an input over the bound of the hook: its one
// read buffer, the copy of a field the hook reads, the copy of a member
// name it compares, and the nesting the decoder of the standard library
// accepts.
const (
	readBuffer = 64 << 10
	fieldBound = 64 << 10
	nameBound  = 256
	maxDepth   = 10000
)

// scanner reads one JSON document in bounded memory, through one fixed
// read buffer, keeping a copy of the value it reads only while keep is set.
type scanner struct {
	r    *bufio.Reader
	keep *copied
}

// token is the next byte that is not white space.
func (s *scanner) token() (byte, error) {
	for {
		b, err := s.r.ReadByte()
		if err != nil {
			return 0, readFailure(err)
		}
		if !jsonSpace(b) {
			return b, nil
		}
	}
}

func jsonSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' }

// end requires nothing but white space after the document.
func (s *scanner) end() error {
	for {
		b, err := s.r.ReadByte()
		switch {
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return readFailure(err)
		case !jsonSpace(b):
			return malformed("the end of the input after its object")
		}
	}
}

// value reads one value whose first byte was read.
func (s *scanner) value(first byte, depth int) error {
	switch {
	case first == '{' || first == '[':
		return s.container(first, depth+1)
	case first == '"':
		return s.text(nil)
	case first == '-' || first >= '0' && first <= '9':
		return s.number(first)
	case first == 't':
		return s.literal("true")
	case first == 'f':
		return s.literal("false")
	case first == 'n':
		return s.literal("null")
	default:
		return malformed("the beginning of a value")
	}
}

// container reads an object or an array whose opening byte was read.
func (s *scanner) container(open byte, depth int) error {
	if depth > maxDepth {
		return malformed("a nesting the decoder accepts")
	}
	s.keep.add(open)
	if open == '{' {
		return s.members(func(_ string, first byte) error { return s.value(first, depth) })
	}
	return s.sequence(']', func(first byte) error { return s.value(first, depth) })
}

// members reads the members of an object whose opening brace was read,
// handing each its name, decoded when no copy is kept, and the first byte
// of its value, which each reads.
func (s *scanner) members(each func(string, byte) error) error {
	return s.sequence('}', func(first byte) error { return s.member(first, each) })
}

// sequence reads the items of a container whose opening byte was read, up
// to its closing byte, each item read by item from its first byte.
func (s *scanner) sequence(closing byte, item func(byte) error) error {
	b, err := s.token()
	if err == nil && b == closing {
		s.keep.add(b)
		return nil
	}
	for err == nil {
		if err = item(b); err != nil {
			break
		}
		if b, err = s.token(); err != nil {
			break
		}
		s.keep.add(b)
		if b == closing {
			return nil
		}
		if b != ',' {
			return malformed("a comma or the end of a container")
		}
		b, err = s.token()
	}
	return err
}

// member reads one member name, its colon and its value, the first byte of
// the name read.
func (s *scanner) member(first byte, each func(string, byte) error) error {
	name, err := s.label(first)
	if err != nil {
		return err
	}
	b, err := s.token()
	if err != nil {
		return err
	}
	if b != ':' {
		return malformed("a colon after a member name")
	}
	s.keep.add(b)
	if b, err = s.token(); err != nil {
		return err
	}
	return each(name, b)
}

// literal reads true, false or null, whose first byte was read.
func (s *scanner) literal(word string) error {
	s.keep.add(word[0])
	for i := 1; i < len(word); i++ {
		b, err := s.r.ReadByte()
		if err != nil {
			return readFailure(err)
		}
		if b != word[i] {
			return malformed("a literal")
		}
		s.keep.add(b)
	}
	return nil
}

// numberForm is the most bytes the form of a number a reader checks may
// hold: a valid one has at most a sign, two digits, a point, two digits,
// an exponent letter, a sign and two digits.
const numberForm = 16

// number reads a number whose first byte was read, in constant memory
// whatever its length: the run of bytes a number may hold is checked by
// the standard library in a form whose every run of digits is cut to two,
// which keeps each rule of the grammar, a leading zero and a part without
// digits included. A number ends before any other byte, and no valid
// document holds one of these bytes right after a number.
func (s *scanner) number(first byte) error {
	form, run := []byte{first}, digitRun(0, first)
	s.keep.add(first)
	for {
		b, err := s.r.ReadByte()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return readFailure(err)
		}
		if !strings.ContainsRune("0123456789+-.eE", rune(b)) {
			if unread := s.r.UnreadByte(); unread != nil {
				return readFailure(unread)
			}
			break
		}
		s.keep.add(b)
		if run = digitRun(run, b); run <= 2 && len(form) <= numberForm {
			form = append(form, b)
		}
	}
	if len(form) > numberForm || !json.Valid(form) {
		return malformed("a number")
	}
	return nil
}

// digitRun is the length of the run of digits a byte ends.
func digitRun(run int, b byte) int {
	if b >= '0' && b <= '9' {
		return run + 1
	}
	return 0
}
