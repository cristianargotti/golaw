package codex

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// unquoting is the decoding of a string read in slices: whether a
// backslash was read, the digits of a unicode escape still to read with
// its value so far, a surrogate half held for the escape that may complete
// it, and the decoded bytes of the current slice when they are wanted.
type unquoting struct {
	decode  bool
	escaped bool
	digits  int
	code    rune
	held    rune
	out     []byte
}

// copied is the raw copy of a value without its white space, up to its
// limit; a longer value keeps nothing and is marked over.
type copied struct {
	raw   []byte
	limit int
	over  bool
}

func (c *copied) add(b ...byte) {
	if c == nil || c.over {
		return
	}
	if len(c.raw)+len(b) > c.limit {
		c.raw, c.over = nil, true
		return
	}
	c.raw = append(c.raw, b...)
}

// text reads a string whose opening quote was read, one slice of the read
// buffer at a time: its escapes and bytes checked as the standard decoder
// checks them, its raw form added to the copy while one is kept, and its
// bytes decoded for sink, slice by slice, only when sink is set.
func (s *scanner) text(sink func([]byte)) error {
	s.keep.add('"')
	u := unquoting{decode: sink != nil}
	for {
		chunk, err := s.r.ReadSlice('"')
		if err != nil && !errors.Is(err, bufio.ErrBufferFull) {
			return readFailure(err)
		}
		s.keep.add(chunk...)
		done, scanErr := u.scan(chunk)
		if scanErr != nil {
			return scanErr
		}
		if len(u.out) > 0 {
			sink(u.out)
			u.out = u.out[:0]
		}
		if done {
			return nil
		}
	}
}

// scan decodes one slice of a string from the state the slice before it
// ended in, and answers whether a quote outside an escape ended the
// string; the slice holds no byte past that quote.
func (u *unquoting) scan(chunk []byte) (bool, error) {
	for len(chunk) > 0 {
		if n := u.plain(chunk); n > 0 {
			chunk = chunk[n:]
			continue
		}
		if done, err := u.special(chunk[0]); done || err != nil {
			return done, err
		}
		chunk = chunk[1:]
	}
	return false, nil
}

// plain takes the run of plain bytes that starts a slice outside an
// escape, in one append, and answers its length.
func (u *unquoting) plain(chunk []byte) int {
	if u.escaped || u.digits > 0 {
		return 0
	}
	// The search runs in the standard library, so an input of megabytes
	// costs no instrumented step per byte.
	n := bytes.IndexAny(chunk, stopBytes)
	if n < 0 {
		n = len(chunk)
	}
	if n > 0 {
		u.settle()
		u.put(chunk[:n]...)
	}
	return n
}

// special reads one byte that is no plain text: a digit or the byte of an
// escape, a backslash, the closing quote, or a control character.
func (u *unquoting) special(c byte) (bool, error) {
	switch {
	case u.digits > 0:
		return false, u.digit(c)
	case u.escaped:
		return false, u.escape(c)
	case c == '\\':
		u.escaped = true
		return false, nil
	case c == '"':
		u.settle()
		return true, nil
	default:
		return false, malformed("a control character in a string")
	}
}

// escape reads the byte after a backslash.
func (u *unquoting) escape(c byte) error {
	u.escaped = false
	if c == 'u' {
		u.digits, u.code = unicodeDigits, 0
		return nil
	}
	b, ok := escapedByte(c)
	if !ok {
		return malformed("an escape in a string")
	}
	u.settle()
	u.put(b)
	return nil
}

// escapedByte is the byte a one-byte escape stands for.
func escapedByte(c byte) (byte, bool) {
	if at := strings.IndexByte(`"\/bfnrt`, c); at >= 0 {
		return "\"\\/\b\f\n\r\t"[at], true
	}
	return 0, false
}

// digit reads one hexadecimal digit of a unicode escape, in either case.
func (u *unquoting) digit(c byte) error {
	var v rune
	switch {
	case c >= '0' && c <= '9':
		v = rune(c - '0')
	case c >= 'a' && c <= 'f':
		v = rune(c-'a') + 10
	case c >= 'A' && c <= 'F':
		v = rune(c-'A') + 10
	default:
		return malformed("a unicode escape in a string")
	}
	u.code = u.code<<hexBits | v
	u.digits--
	if u.digits == 0 {
		u.unicode(u.code)
	}
	return nil
}

// The digits of a unicode escape and the bits of one digit, and the bytes
// that end a run of plain text in a string: the quote, the backslash and
// the control characters the grammar refuses.
const (
	unicodeDigits = 4
	hexBits       = 4
	stopBytes     = "\"\\\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0a\x0b\x0c\x0d\x0e\x0f" +
		"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f"
)

// unicode takes the rune of a unicode escape. A surrogate half is held for
// the escape that may follow it, and a half that stays alone becomes the
// replacement character, as the standard decoder pairs them.
func (u *unquoting) unicode(r rune) {
	if held := u.held; held != 0 {
		u.held = 0
		if pair := utf16.DecodeRune(held, r); pair != unicode.ReplacementChar {
			u.emit(pair)
			return
		}
		u.emit(unicode.ReplacementChar)
	}
	if utf16.IsSurrogate(r) {
		u.held = r
		return
	}
	u.emit(r)
}

// settle writes a held surrogate half that no escape completed.
func (u *unquoting) settle() {
	if u.held != 0 {
		u.held = 0
		u.emit(unicode.ReplacementChar)
	}
}

func (u *unquoting) put(b ...byte) {
	if u.decode {
		u.out = append(u.out, b...)
	}
}

func (u *unquoting) emit(r rune) {
	if u.decode {
		u.out = utf8.AppendRune(u.out, r)
	}
}
