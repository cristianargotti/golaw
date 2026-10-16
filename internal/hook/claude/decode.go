package claude

import (
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// Decode keeps vendor payloads outside the policy engine.
func Decode(data []byte, base hook.Base, snapshot model.Snapshot) (hook.Request, error) {
	w, event, in, err := parse(data)
	if err != nil {
		return hook.Request{}, err
	}
	return decode(w, event, in, base, snapshot)
}

// decode is the request of an event from its one parse.
func decode(w wire, event model.Event, in toolInput, base hook.Base, snapshot model.Snapshot) (hook.Request, error) {
	if event.Shape == model.ShapePost {
		return postRequest(w, event, in, base)
	}
	switch event.Tool {
	case writeTool, editTool, notebookTool:
		return writeRequest(w, event, in, base, snapshot)
	case readTool:
		return readRequest(w, event, in, base)
	case bashTool, powerTool:
		return shellRequest(event, in)
	default:
		return hook.Request{}, fmt.Errorf("%w: unsupported Claude tool", model.ErrCannotJudge)
	}
}

// readRequest names a read inside the repository by its path from the
// root, so the directories a clone sits under never reach the name rule; a
// path outside the repository, a credential store of the home, keeps the
// name it was given. A link between the name and the file gives the read
// a second name, the file the link leads to, named the same way or by its
// absolute path outside, and the rule judges both.
func readRequest(w wire, event model.Event, in toolInput, base hook.Base) (hook.Request, error) {
	if in.err != nil {
		return hook.Request{}, fmt.Errorf("%w: Read input: %w", model.ErrCannotJudge, in.err)
	}
	path := in.target
	if path == "" {
		return hook.Request{}, fmt.Errorf("%w: Read path is absent", model.ErrCannotJudge)
	}
	read := model.ReadRequest{Path: readName(base.Requested, w.Cwd, path, path)}
	if target := readName(base.Relative, w.Cwd, path, base.Resolve(w.Cwd, path)); target != read.Path {
		read.Real = target
	}
	event.Shape = model.ShapeRead
	event.Read = &read
	return hook.Request{Event: event}, nil
}

// readName is a name of a read from the root when the repository holds it,
// else the given path, in slashes; a fallback, never an error, because a
// read outside the repository is still judged by its name.
func readName(inside func(string, string) (string, error), cwd, name, outside string) string {
	if relative, err := inside(cwd, name); err == nil { // fallback: a name outside the repository keeps its own spelling
		return relative
	}
	return strings.ReplaceAll(outside, `\`, "/")
}

func shellRequest(event model.Event, in toolInput) (hook.Request, error) {
	if in.err != nil {
		return hook.Request{}, fmt.Errorf("%w: shell input: %w", model.ErrCannotJudge, in.err)
	}
	if in.command == nil {
		return hook.Request{}, fmt.Errorf("%w: shell command is absent", model.ErrCannotJudge)
	}
	event.Shape = model.ShapeShell
	event.Shell = &model.ShellCommand{
		Command: *in.command, Cwd: event.Cwd, Tool: event.Tool,
	}
	return hook.Request{Event: event}, nil
}
