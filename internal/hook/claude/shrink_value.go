package claude

import (
	"bufio"
	"errors"
	"io"
)

// The states of a string scan: outside an escape, after its backslash, and
// inside the four digits of a unicode escape, from its first digit on.
const (
	plainByte  = 0
	afterSlash = 1
	firstDigit = 2
	lastDigit  = 5
)

// str reads a string whose opening quote was read, checking its escapes
// and its bytes as the decoder does, one slice of the buffer at a time,
// and writes it to the copy when its raw text fits in limit, or the elided
// string in its place.
func (s *shrinker) str(limit int) error {
	start := len(s.out)
	s.out = append(s.out, '"')
	held, state := true, plainByte
	for {
		chunk, err := s.r.ReadSlice('"')
		if err != nil && !errors.Is(err, bufio.ErrBufferFull) {
			return ended(err)
		}
		var done bool
		if state, done, err = scanString(chunk, state); err != nil {
			return err
		}
		held = held && s.hold(start, chunk, limit)
		if done {
			if !held {
				s.out = append(s.out, elided...)
			}
			return nil
		}
	}
}

// hold adds a slice of a string to the copy while the string fits in
// limit, and drops what the copy held of it once it does not.
func (s *shrinker) hold(start int, chunk []byte, limit int) bool {
	s.out = append(s.out, chunk...)
	if len(s.out)-start <= limit+2 {
		return true
	}
	s.out = s.out[:start]
	return false
}

// scanString checks a slice of a string from the state the slice before it
// ended in, and answers the state at its end and whether a quote outside
// an escape ended the string there; the slice holds no byte past it.
func scanString(chunk []byte, state int) (int, bool, error) {
	for _, c := range chunk {
		switch {
		case state == afterSlash:
			state = escaped(c)
			if state < 0 {
				return 0, false, invalid("an escape in a string")
			}
		case state >= firstDigit:
			if !hexDigit(c) {
				return 0, false, invalid("a unicode escape in a string")
			}
			state = (state + 1) % (lastDigit + 1)
		case c == '\\':
			state = afterSlash
		case c == '"':
			return plainByte, true, nil
		case c < ' ':
			return 0, false, invalid("a control character in a string")
		}
	}
	return state, false, nil
}

// escaped is the state after the byte that follows a backslash: back
// outside the escape for an escape of one byte, at the first digit of a
// unicode escape, and below zero for a byte no escape takes.
func escaped(c byte) int {
	switch c {
	case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
		return plainByte
	case 'u':
		return firstDigit
	default:
		return -1
	}
}

func hexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// number reads a number whose first byte was read and writes it to the
// copy once the grammar accepts it.
func (s *shrinker) number(first byte) error {
	text := []byte{first}
	for {
		b, err := s.r.ReadByte()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return ended(err)
		}
		if !numberByte(b) {
			if err := s.r.UnreadByte(); err != nil {
				return ended(err)
			}
			break
		}
		text = append(text, b)
	}
	if !validNumber(text) {
		return invalid("a number")
	}
	s.out = append(s.out, text...)
	return nil
}

func numberByte(b byte) bool {
	return b >= '0' && b <= '9' || b == '-' || b == '+' || b == '.' || b == 'e' || b == 'E'
}

// validNumber reports a number the grammar accepts: an optional minus, an
// integer without a leading zero, then a fraction and an exponent, each
// with its digits.
func validNumber(text []byte) bool {
	i := integerEnd(text)
	if i >= 0 {
		i = fractionEnd(text, i)
	}
	if i >= 0 {
		i = exponentEnd(text, i)
	}
	return i == len(text)
}

// integerEnd is the index past the integer part of a number, its minus
// included, or -1 when it has none.
func integerEnd(text []byte) int {
	i := 0
	if i < len(text) && text[i] == '-' {
		i++
	}
	switch {
	case i < len(text) && text[i] == '0':
		return i + 1
	case i < len(text) && text[i] >= '1' && text[i] <= '9':
		return digits(text, i)
	default:
		return -1
	}
}

// fractionEnd is the index past a fraction at i, i itself without one, or
// -1 for a point without digits.
func fractionEnd(text []byte, i int) int {
	if i >= len(text) || text[i] != '.' {
		return i
	}
	if end := digits(text, i+1); end > i+1 {
		return end
	}
	return -1
}

// exponentEnd is the index past an exponent at i, i itself without one, or
// -1 for an exponent without digits.
func exponentEnd(text []byte, i int) int {
	if i >= len(text) || text[i] != 'e' && text[i] != 'E' {
		return i
	}
	i++
	if i < len(text) && (text[i] == '+' || text[i] == '-') {
		i++
	}
	if end := digits(text, i); end > i {
		return end
	}
	return -1
}

// digits is the index past the run of digits from i.
func digits(text []byte, i int) int {
	for i < len(text) && text[i] >= '0' && text[i] <= '9' {
		i++
	}
	return i
}

// literal reads true, false or null, whose first byte was read.
func (s *shrinker) literal(first byte) error {
	word := map[byte]string{'t': "true", 'f': "false", 'n': "null"}[first]
	rest := make([]byte, len(word)-1)
	if _, err := io.ReadFull(s.r, rest); err != nil || string(rest) != word[1:] {
		return invalid("a literal")
	}
	s.out = append(s.out, word...)
	return nil
}
