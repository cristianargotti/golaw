package cursor

import (
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// pathField is the file a beforeReadFile event asks for (D17).
const pathField = "file_path"

// withReadTarget gives a read event the file it names as its target, so
// enrollment looks the read up where the file lives (A3) before the session's
// working directory; a name that is no string leaves the target empty and
// the decoder refuses the event.
func withReadTarget(input payload, event model.Event) model.Event {
	if event.Shape == model.ShapeRead {
		event.Target = input.text(pathField)
	}
	return event
}

// readRequest is the request of a beforeReadFile event, judged by name only
// (D17): the content and the attachments Cursor sends with it never reach
// the event, so no rule, record or reason can hold what the file says. A
// name inside the repository is spelled from the root, so the directories a
// clone sits under never reach the name rule; a name outside keeps its own
// spelling, because a credential store of the home lives there. A link
// gives the read a second name, the file it leads to, and the rule judges
// both.
func readRequest(input payload, event model.Event, base hook.Base) (hook.Request, error) {
	path, err := input.required(pathField)
	if err != nil {
		return hook.Request{}, err
	}
	if path == "" {
		return hook.Request{}, fmt.Errorf("%w: cursor %s is empty", model.ErrCannotJudge, pathField)
	}
	read := model.ReadRequest{Path: readName(base.Requested, event.Cwd, path, path)}
	if resolved := readName(base.Relative, event.Cwd, path, base.Resolve(event.Cwd, path)); resolved != read.Path {
		read.Real = resolved
	}
	event.Read = &read
	return hook.Request{Event: event}, nil
}

// readName is one name of a read: from the root when the repository holds
// it, else the outside spelling in slashes.
func readName(inside func(string, string) (string, error), cwd, name, outside string) string {
	if relative, err := inside(cwd, name); err == nil { // fallback: a name outside the repository is judged by its own spelling
		return relative
	}
	return strings.ReplaceAll(outside, `\`, "/")
}
