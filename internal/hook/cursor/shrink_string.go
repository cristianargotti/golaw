package cursor

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

// text copies a string whose opening quote was read: the whole string when
// its text fits in limit bytes, else the elision in its place. The string
// is read one slice of the buffer at a time, each slice ending at the next
// quote, so a long string is checked without ever being held.
func (r *reducer) text(limit int) error {
	start := len(r.out)
	r.out = append(r.out, '"')
	var scan quoted
	for size := 0; ; {
		slice, err := r.in.ReadSlice('"')
		if err != nil && !errors.Is(err, bufio.ErrBufferFull) {
			return cut(fmt.Errorf("read input: %w", err))
		}
		closed, err := scan.feed(slice)
		if err != nil {
			return err
		}
		size += len(slice)
		if size <= limit+1 {
			r.out = append(r.out, slice...)
		}
		if closed {
			if size > limit+1 {
				r.out = append(r.out[:start], elision...)
			}
			return nil
		}
	}
}

// unicodeDigits are the hexadecimal digits a unicode escape takes.
const unicodeDigits = 4

// quoted is where a string scan stands between two slices: right after the
// backslash of an escape, or with hexadecimal digits of a unicode escape
// still to come.
type quoted struct {
	escape bool
	digits int
}

// feed checks a slice of a string, which holds no byte past the first quote
// it reaches, and answers whether that quote closes the string. A unicode
// escape of a lone surrogate is taken, as encoding/json takes it, so the
// reducer refuses nothing the parse of an event within the bound accepts.
func (q *quoted) feed(slice []byte) (bool, error) {
	for _, c := range slice {
		switch {
		case q.digits > 0:
			if !isHex(c) {
				return false, malformed("a unicode escape")
			}
			q.digits--
		case q.escape:
			q.escape = false
			if err := q.escaped(c); err != nil {
				return false, err
			}
		case c == '\\':
			q.escape = true
		case c == '"':
			return true, nil
		case c < ' ':
			return false, malformed("a control character in a string")
		}
	}
	return false, nil
}

// escaped reads the byte after the backslash of an escape.
func (q *quoted) escaped(c byte) error {
	switch c {
	case 'u':
		q.digits = unicodeDigits
	case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
	default:
		return malformed("an escape in a string")
	}
	return nil
}

func isHex(c byte) bool {
	return isDigit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// significant is the next byte that is not white space.
func (r *reducer) significant() (byte, error) {
	for {
		b, err := r.next()
		if err != nil {
			return 0, cut(err)
		}
		if !space(b) {
			return b, nil
		}
	}
}

// finish requires nothing but white space after the document.
func (r *reducer) finish() error {
	for {
		b, err := r.next()
		switch {
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return cut(err)
		case !space(b):
			return malformed("a value after the document")
		}
	}
}

// next is the next byte of the input, the one a number read past its end
// first.
func (r *reducer) next() (byte, error) {
	if r.carried {
		r.carried = false
		return r.carry, nil
	}
	b, err := r.in.ReadByte()
	if err != nil {
		return 0, fmt.Errorf("read input: %w", err)
	}
	return b, nil
}

func space(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' }
