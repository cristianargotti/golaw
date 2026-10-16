package claude

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// Decode keeps vendor payloads outside the policy engine.
func Decode(data []byte, root string, snapshot model.Snapshot) (hook.Request, error) {
	event, err := Header(data)
	if err != nil {
		return hook.Request{}, err
	}
	w, err := envelope(data)
	if err != nil {
		return hook.Request{}, err
	}
	if event.Shape == model.ShapePost {
		return postRequest(w, event, root)
	}
	switch event.Tool {
	case writeTool, editTool, notebookTool:
		return Write(data, root, snapshot)
	case readTool:
		return readRequest(w, event, root)
	case bashTool, powerTool:
		return shellRequest(w, event)
	default:
		return hook.Request{}, fmt.Errorf("%w: unsupported Claude tool", model.ErrCannotJudge)
	}
}

// readRequest names a read inside the repository by its path from the
// root, so the directories a clone sits under (a secrets folder in the
// home) never reach the name rule; a path outside the repository, a
// credential store of the home, keeps the name it was given.
func readRequest(w wire, event model.Event, root string) (hook.Request, error) {
	var input struct {
		Path string `json:"file_path"`
	}
	if err := json.Unmarshal(w.Input, &input); err != nil {
		return hook.Request{}, fmt.Errorf("%w: Read input: %w", model.ErrCannotJudge, err)
	}
	if input.Path == "" {
		return hook.Request{}, fmt.Errorf("%w: Read path is absent", model.ErrCannotJudge)
	}
	path := strings.ReplaceAll(input.Path, `\`, "/")
	if relative, err := hook.Relative(root, w.Cwd, input.Path); err == nil { // fallback: a path outside the repository keeps its own name
		path = relative
	}
	event.Shape = model.ShapeRead
	event.Read = &model.ReadRequest{Path: path}
	return hook.Request{Event: event}, nil
}

func shellRequest(w wire, event model.Event) (hook.Request, error) {
	var input struct {
		Command *string `json:"command"`
	}
	if err := json.Unmarshal(w.Input, &input); err != nil {
		return hook.Request{}, fmt.Errorf("%w: shell input: %w", model.ErrCannotJudge, err)
	}
	if input.Command == nil {
		return hook.Request{}, fmt.Errorf("%w: shell command is absent", model.ErrCannotJudge)
	}
	event.Shape = model.ShapeShell
	event.Shell = &model.ShellCommand{
		Command: *input.Command, Cwd: event.Cwd, Tool: event.Tool,
	}
	return hook.Request{Event: event}, nil
}
