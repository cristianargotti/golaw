package cursor

import (
	"bufio"
	"fmt"
	"io"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// The bounds of the reducer: a string the hook judges by (the event, an
// identity, a root, a path or a command) is kept up to keptField bytes and
// any other up to keptText; the containers nest at most maxDepth deep, the
// root included, which is the limit of encoding/json, so the reducer never
// refuses a document the parse of an event within the bound would take.
const (
	keptField = 64 << 10
	keptText  = 4 << 10
	maxDepth  = 10000
	readChunk = 64 << 10
)

// elision is the string the copy holds in place of one the reducer does not
// keep: it starts with a NUL, which no path, command or identity can hold,
// so a field the hook judges by is told from any value it could carry.
const elision = `"\u0000elided over the input bound"`

// contentField is the file text beforeReadFile carries, which no route reads.
const contentField = "content"

// What the reducer expects at the next byte that is not white space.
const (
	expectValue  = iota // any value
	expectItem          // a value or the end of an empty array
	expectMember        // a key or the end of an empty object
	expectKey           // a key after a comma
	expectColon         // the colon after a key
	expectNext          // a comma or the end of the open container
	expectEnd           // nothing but white space up to the end of the input
)

// container is one open object or array: the byte that closes it and, for
// an array, the bound its strings keep, the one of the key it is the value
// of, so the roots of workspace_roots keep the bound of a root.
type container struct {
	closer byte
	limit  int
}

// reducer copies one JSON document through a bounded buffer. A number ends
// at the byte after it, which the reducer carries to the grammar instead of
// putting it back.
type reducer struct {
	in      *bufio.Reader
	out     []byte
	open    []container
	member  string
	carry   byte
	carried bool
}

// shrink reads one JSON document to the end of the input in bounded memory
// and answers a copy of it that keeps every key and the structure around
// them, with each string longer than its bound elided and the read content
// dropped. The whole grammar is checked as encoding/json checks it, so the
// copy decodes as the input would but for what it elides, and a copy that
// still holds more than the hook reads is refused.
func shrink(input io.Reader) ([]byte, error) {
	r := reducer{in: bufio.NewReaderSize(input, readChunk)}
	for expect := expectValue; expect != expectEnd; {
		b, err := r.significant()
		if err != nil {
			return nil, err
		}
		if expect, err = r.accept(expect, b); err != nil {
			return nil, err
		}
		if len(r.out) > hook.MaxInputBytes {
			return nil, fmt.Errorf("%w: an input over 8 MiB holds more than the hook reads outside its long strings", model.ErrCannotJudge)
		}
	}
	if err := r.finish(); err != nil {
		return nil, err
	}
	return r.out, nil
}

// accept reads what b begins where the grammar expects it and answers what
// the grammar expects next.
func (r *reducer) accept(expect int, b byte) (int, error) {
	switch expect {
	case expectColon:
		if b != ':' {
			return 0, malformed("a colon after a key")
		}
		r.out = append(r.out, b)
		return expectValue, nil
	case expectNext:
		return r.separate(b)
	case expectMember:
		if b == '}' {
			return r.shut(b), nil
		}
		return r.key(b)
	case expectKey:
		return r.key(b)
	case expectItem:
		if b == ']' {
			return r.shut(b), nil
		}
	}
	return r.value(b)
}

// value reads a value whose first byte was read, its strings kept by the
// bound of where it stands.
func (r *reducer) value(b byte) (int, error) {
	limit := r.valueLimit()
	var err error
	switch {
	case b == '{' || b == '[':
		return r.push(b, limit)
	case b == '"':
		err = r.text(limit)
	case b == '-' || isDigit(b):
		err = r.number(b)
	case b == 't' || b == 'f' || b == 'n':
		err = r.literal(b)
	default:
		err = malformed("the start of a value")
	}
	return r.after(), err
}

// key reads the key of a member whose opening quote was read; the key names
// the bound of the value that follows it.
func (r *reducer) key(b byte) (int, error) {
	if b != '"' {
		return 0, malformed("a key")
	}
	start := len(r.out)
	if err := r.text(keptField); err != nil {
		return 0, err
	}
	r.member = string(r.out[start+1 : len(r.out)-1])
	return expectColon, nil
}

// push opens a container, refusing one past the nesting limit.
func (r *reducer) push(b byte, limit int) (int, error) {
	if len(r.open) == maxDepth {
		return 0, fmt.Errorf("%w: an input over 8 MiB nests more than %d containers", model.ErrCannotJudge, maxDepth)
	}
	r.out = append(r.out, b)
	if b == '{' {
		r.open = append(r.open, container{closer: '}'})
		return expectMember, nil
	}
	r.open = append(r.open, container{closer: ']', limit: limit})
	return expectItem, nil
}

// separate reads what follows a value inside a container: a comma, or the
// byte that closes that container.
func (r *reducer) separate(b byte) (int, error) {
	top := r.open[len(r.open)-1]
	switch {
	case b == top.closer:
		return r.shut(b), nil
	case b != ',':
		return 0, malformed("a comma or the end of a container")
	}
	r.out = append(r.out, b)
	if top.closer == '}' {
		return expectKey, nil
	}
	return expectValue, nil
}

// shut closes the innermost container.
func (r *reducer) shut(b byte) int {
	r.open = r.open[:len(r.open)-1]
	r.out = append(r.out, b)
	return r.after()
}

// after is what the grammar expects once a value is complete.
func (r *reducer) after() int {
	if len(r.open) == 0 {
		return expectEnd
	}
	return expectNext
}

// valueLimit is the bound of a string at the current place: the one of its
// key inside an object, the one of the array it is an item of, and the
// shorter one at the root.
func (r *reducer) valueLimit() int {
	depth := len(r.open)
	if depth == 0 {
		return keptText
	}
	if top := r.open[depth-1]; top.closer == ']' {
		return top.limit
	}
	return kept(r.member, depth)
}

// kept is the bound of the string value of a key at a depth: the fields a
// route judges by keep the longer bound, the content of a read is dropped
// whatever its length, and every other string keeps the shorter one. A key
// spelled with escapes reads as another key and keeps the shorter bound,
// which only ever elides more and refuses the event.
func kept(key string, depth int) int {
	switch key {
	case contentField:
		if depth == 1 {
			return 0
		}
	case nameField, toolField, conversationField, "generation_id", "tool_use_id",
		"cwd", "workspace_roots", pathField, commandField:
		return keptField
	}
	return keptText
}
