package claude

import (
	"encoding/json"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// toolInput is the tool input of an event decoded once, the way its tool
// reads it: the path it names, the fields of a write or an edit and the
// command of a shell tool, with what the decoding met, which the decoder
// answers once the event is enrolled.
type toolInput struct {
	target  string
	write   writeInput
	edit    editInput
	command *string
	err     error
}

// pathFields are the fields of a tool input that name a file; a field that
// is not a string names nothing.
type pathFields struct {
	File     string `json:"file_path"`
	Notebook string `json:"notebook_path"`
}

// Parse decodes an event once: the envelope, the header enrollment reads
// and the tool input as its tool reads it, so the decoder of the request
// reads none of the bytes again.
func Parse(data []byte) (hook.Parsed, error) {
	w, event, in, err := parse(data)
	if err != nil {
		return hook.Parsed{}, err
	}
	return hook.Parsed{Event: event, Decode: func(base hook.Base, snapshot model.Snapshot) (hook.Request, error) {
		return decode(w, event, in, base, snapshot)
	}}, nil
}

func parse(data []byte) (wire, model.Event, toolInput, error) {
	w, err := envelope(data)
	if err != nil {
		return wire{}, model.Event{}, toolInput{}, err
	}
	in := readInput(w)
	event, err := header(w, in.target)
	if err != nil {
		return wire{}, model.Event{}, toolInput{}, err
	}
	return w, event, in, nil
}

// readInput decodes the tool input of an event once. A decoding goes on
// past a field of another type or, for a write, an unknown one, so the path
// a refused write names is still where its enrollment is looked up.
func readInput(w wire) toolInput {
	pre := w.Name == preEvent
	var in toolInput
	switch {
	case pre && w.Tool == writeTool:
		in.err = strictInput(w.Input, &in.write)
		in.target = in.write.Path
	case pre && w.Tool == editTool:
		in.err = strictInput(w.Input, &in.edit)
		in.target = in.edit.Path
	case pre && (w.Tool == bashTool || w.Tool == powerTool):
		var shell struct {
			Command *string `json:"command"`
		}
		in.err = json.Unmarshal(w.Input, &shell)
		in.command = shell.Command
	case w.Tool == readTool:
		var read struct {
			File string `json:"file_path"`
		}
		in.err = json.Unmarshal(w.Input, &read)
		in.target = read.File
	case w.Tool == writeTool || w.Tool == editTool || w.Tool == notebookTool:
		var named pathFields
		in.err = json.Unmarshal(w.Input, &named)
		in.target = named.File
		if w.Tool == notebookTool {
			in.target = named.Notebook
		}
	}
	return in
}
