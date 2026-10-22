package codex

import (
	"bytes"
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// The bounds of the skeleton of a patch over the bound: a line kept whole,
// the bytes of the end of a line a stub keeps, and the length up to which a
// content line is its own stub.
const (
	lineBound = 64 << 10
	tailKeep  = 8
	stubBound = 2 * tailKeep
)

// skeleton is a patch read line by line as its command streams in, in
// bounded memory, so the pinned grammar reader validates it and names its
// files without its content. A line that may be a marker or a header,
// whatever the hunk around it, is kept whole, as is the first line that is
// not blank; every other line keeps only what its role in the grammar
// reads: the empty line, its first character, whether it is blank, and,
// for the last line that is not blank, the end the here-document wrapper
// tests. The pending range is that last content line, reduced to its
// first character once a later line shows it is not the last.
type skeleton struct {
	text    []byte
	line    []byte
	long    *longLine
	started bool
	pending [2]int
	err     error
}

// longLine is a line that outgrew lineBound: its first character, whether
// it is blank so far, and its last bytes.
type longLine struct {
	first []byte
	blank bool
	tail  []byte
}

// write takes the next decoded bytes of the command.
func (k *skeleton) write(p []byte) {
	for k.err == nil && len(p) > 0 {
		end := bytes.IndexByte(p, '\n')
		if end < 0 {
			k.grow(p)
			return
		}
		k.grow(p[:end])
		k.finish()
		p = p[end+1:]
	}
}

// grow adds bytes to the current line; a line that outgrows the bound is a
// content line reduced as it streams, or a header the hook cannot keep.
func (k *skeleton) grow(p []byte) {
	switch {
	case k.err != nil:
	case k.long != nil:
		k.long.feed(p)
	case len(k.line)+len(p) <= lineBound:
		k.line = append(k.line, p...)
	default:
		k.line = append(k.line, p...)
		k.outgrow()
	}
}

// outgrow turns the current line, past lineBound, into a long line.
func (k *skeleton) outgrow() {
	if headerLike(k.line) {
		k.err = fmt.Errorf("%w: a header line of the patch over the 8 MiB the hook reads is longer than the %d bytes it keeps", model.ErrCannotJudge, lineBound)
		return
	}
	k.long = &longLine{first: bytes.Clone(firstRune(k.line)), blank: len(bytes.TrimSpace(k.line)) == 0}
	k.long.feed(k.line[len(k.line)-2*tailKeep:])
	k.line = k.line[:0]
}

// feed takes the next bytes of a long line; its blank state reads them by
// byte, so a long run of multibyte white space reads as text, a stated
// limit that only refuses.
func (l *longLine) feed(p []byte) {
	if l.blank {
		l.blank = len(bytes.TrimLeft(p, " \t\n\r\v\f")) == 0
	}
	l.tail = append(l.tail, p...)
	if len(l.tail) > 2*tailKeep {
		l.tail = append(l.tail[:0], l.tail[len(l.tail)-2*tailKeep:]...)
	}
}

// finish ends the current line.
func (k *skeleton) finish() {
	switch {
	case k.err != nil:
	case k.long != nil:
		k.content(k.long.first, k.long.blank, nil, k.long.tail)
		k.long = nil
	default:
		k.keep(k.line)
	}
	k.line = k.line[:0]
}

// keep adds a line read whole by its role.
func (k *skeleton) keep(line []byte) {
	blank := len(bytes.TrimSpace(line)) == 0
	switch {
	case headerLike(line):
		k.settle()
		k.add(validText(line))
		k.started = true
	case !blank && !k.started:
		k.add(validText(line))
		k.started = true
	default:
		k.content(firstRune(line), blank, line, line)
	}
}

// content adds the stub of a content line from its first character, its
// blank state, the line when it was read whole, and its end. A blank line
// keeps its end, white space whose carriage returns the grammar reader
// trims; a line that is not blank becomes the pending last line, with the
// end the here-document wrapper tests. A short line is its own stub.
func (k *skeleton) content(first []byte, blank bool, whole, end []byte) {
	if blank {
		if whole == nil || len(whole) > stubBound {
			whole = append(bytes.Clone(first), lastRunes(end, tailKeep)...)
		}
		k.add(whole)
		return
	}
	k.settle()
	if whole == nil || len(whole) > stubBound {
		whole = append(append(bytes.Clone(first), 'x'), lastRunes(bytes.TrimRightFunc(end, unicode.IsSpace), tailKeep)...)
	}
	k.started = true
	k.pending = [2]int{len(k.text), len(k.text) + len(whole)}
	k.add(whole)
}

// settle reduces the pending line, since a later line shows it is not the
// last, to what its role reads: its first character, and a mark that it
// is not blank when that character is white space. The short form is never
// longer than the line, so the lines after it move in place, once each.
func (k *skeleton) settle() {
	start, end := k.pending[0], k.pending[1]
	if end == 0 {
		return
	}
	short := len(firstRune(k.text[start:end]))
	if r, _ := utf8.DecodeRune(k.text[start:end]); unicode.IsSpace(r) {
		k.text[start+short] = 'x'
		short++
	}
	moved := copy(k.text[start+short:], k.text[end:])
	k.text = k.text[:start+short+moved]
	k.pending = [2]int{}
}

// add appends a line to the skeleton, within the bound of the hook.
func (k *skeleton) add(line []byte) {
	k.text = append(append(k.text, line...), '\n')
	if len(k.text) > hook.MaxInputBytes {
		k.err = fmt.Errorf("%w: the patch over the 8 MiB the hook reads has more lines than the hook keeps: apply it in smaller patches", model.ErrCannotJudge)
	}
}

// close ends the last line and answers the skeleton, lines joined by line
// feeds as the text was.
func (k *skeleton) close() (string, error) {
	k.finish()
	if k.err != nil {
		return "", k.err
	}
	return string(bytes.TrimSuffix(k.text, []byte{'\n'})), nil
}
