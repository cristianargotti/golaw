package claude

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

type writeInput struct {
	Path    string  `json:"file_path"`
	Content *string `json:"content"`
}

// Write reconstructs candidates without executing the intercepted operation.
func Write(data []byte, base hook.Base, snapshot model.Snapshot) (hook.Request, error) {
	event, err := Header(data)
	if err != nil {
		return hook.Request{}, err
	}
	w, err := envelope(data)
	if err != nil {
		return hook.Request{}, err
	}
	if w.Name != preEvent {
		return hook.Request{}, fmt.Errorf("%w: expected pre event", model.ErrCannotJudge)
	}
	if event.Tool == notebookTool {
		return hook.Request{
			Event:  event,
			Denial: "NotebookEdit is not supported by this adapter",
		}, nil
	}
	var candidate model.Candidate
	var content []byte
	switch event.Tool {
	case writeTool:
		candidate, content, err = fullCandidate(w, base, snapshot)
	case editTool:
		candidate, content, err = editCandidate(w, base, snapshot)
	default:
		err = fmt.Errorf("%w: unsupported write tool", model.ErrCannotJudge)
	}
	if err != nil {
		return hook.Request{}, err
	}
	event.Shape = model.ShapeWrite
	event.Candidates = []model.Candidate{candidate}
	return hook.Request{
		Event:   event,
		Content: map[string][]byte{candidate.Path: content},
	}, nil
}

func fullCandidate(w wire, base hook.Base, snapshot model.Snapshot) (model.Candidate, []byte, error) {
	var input writeInput
	if err := strictInput(w.Input, &input); err != nil {
		return model.Candidate{}, nil, err
	}
	if input.Content == nil {
		return model.Candidate{}, nil, fmt.Errorf("%w: Write content is absent", model.ErrCannotJudge)
	}
	path, err := base.Relative(w.Cwd, input.Path)
	if err != nil {
		return model.Candidate{}, nil, fmt.Errorf("%w: Write path: %w", model.ErrCannotJudge, err)
	}
	candidate := model.Candidate{Path: path, Op: model.CandidateCreate, Reconstructed: true}
	if snapshot.Class(path) != model.ClassUnknown {
		base, err := snapshot.Read(path)
		if err != nil {
			return model.Candidate{}, nil, fmt.Errorf("%w: Write base: %w", model.ErrCannotJudge, err)
		}
		candidate.Op = model.CandidateReplace
		candidate.BaseHash = hook.Hash(base)
	}
	return candidate, []byte(*input.Content), nil
}

func strictInput(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: mutation input: %w", model.ErrCannotJudge, err)
	}
	return nil
}
