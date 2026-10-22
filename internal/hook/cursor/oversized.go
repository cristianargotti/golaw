package cursor

import (
	"fmt"
	"io"
	"strings"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// Oversized parses an event whose input is over the bound of the hook from
// the whole input, in bounded memory: the reducer copies it with every
// long string elided and the read content dropped, and the copy goes
// through the same Parse and decoders as any event, so an input they refuse
// within the bound is refused over it. A field a route judges by that the
// copy elided refuses the event rather than being judged as the elision: a
// read is still judged by its name, a Write or Delete still captures its
// key names and is allowed, and a command the hook could not keep cannot be
// judged.
func Oversized(input io.Reader) (hook.Parsed, error) {
	reduced, err := shrink(input)
	if err != nil {
		return hook.Parsed{}, err
	}
	parsed, err := Parse(reduced)
	if err != nil {
		return hook.Parsed{}, err
	}
	event := parsed.Event
	if elidedField(event.Cwd, event.SessionID, event.AgentID, event.ToolUseID, event.Tool, event.Target) {
		return hook.Parsed{}, unkept()
	}
	return oversizedRequest(parsed), nil
}

// oversizedRequest is a parse of an input over the bound whose decoder also
// refuses a request that holds an elided field: the command of a shell
// event, or a name of a read.
func oversizedRequest(parsed hook.Parsed) hook.Parsed {
	decode := parsed.Decode
	parsed.Decode = func(base hook.Base, snapshot model.Snapshot) (hook.Request, error) {
		request, err := decode(base, snapshot)
		if err != nil {
			return hook.Request{}, err
		}
		if elidedField(judgedFields(request.Event)...) {
			return hook.Request{}, unkept()
		}
		return request, nil
	}
	return parsed
}

// judgedFields are the strings of a decoded event a rule judges.
func judgedFields(event model.Event) []string {
	fields := []string{event.Cwd, event.Target}
	if event.Shell != nil {
		fields = append(fields, event.Shell.Command, event.Shell.Cwd)
	}
	if event.Read != nil {
		fields = append(fields, event.Read.Path, event.Read.Real)
	}
	return fields
}

// elidedField reports a field that holds the NUL of the elision, which no
// path, command or identity can hold.
func elidedField(fields ...string) bool {
	for _, field := range fields {
		if strings.ContainsRune(field, 0) {
			return true
		}
	}
	return false
}

// unkept is the refusal of an event over the bound whose judged field is
// longer than the reducer keeps, naming the bound the agent can stay within.
func unkept() error {
	return fmt.Errorf("%w: the input is over 8 MiB and its path, command or identity is longer than the %d KiB the hook keeps of it: shorten it",
		model.ErrCannotJudge, keptField>>10)
}
