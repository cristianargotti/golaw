package claude

import (
	"bytes"
	"encoding/json"
	"errors"
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
	candidate, content, err := candidateOf(w, base, snapshot)
	event.Shape = model.ShapeWrite
	event.Candidates = []model.Candidate{candidate}
	if errors.Is(err, model.ErrTooLarge) && candidate.Path != "" {
		// An edit of a file over the read bound cannot be reconstructed
		// within it, so the edit is left to the pipeline with a notice.
		return hook.Request{Event: event, Gaps: []hook.Gap{hook.OverReadBound(candidate.Path)}}, nil
	}
	if err != nil {
		return hook.Request{}, err
	}
	return hook.Request{
		Event:   event,
		Content: map[string][]byte{candidate.Path: content},
	}, nil
}

// candidateOf reconstructs the candidate of a write tool with its bytes.
func candidateOf(w wire, base hook.Base, snapshot model.Snapshot) (model.Candidate, []byte, error) {
	switch w.Tool {
	case writeTool:
		return fullCandidate(w, base, snapshot)
	case editTool:
		return editCandidate(w, base, snapshot)
	default:
		return model.Candidate{}, nil, fmt.Errorf("%w: unsupported write tool", model.ErrCannotJudge)
	}
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
		candidate.Op = model.CandidateReplace
		base, err := snapshot.Read(path)
		switch {
		case err == nil:
			candidate.BaseHash = hook.Hash(base)
		case !errors.Is(err, model.ErrTooLarge):
			return model.Candidate{}, nil, fmt.Errorf("%w: Write base: %w", model.ErrCannotJudge, err)
		}
		// A base over the read bound binds no hash: the whole content is
		// judged, and the freshness of the base is its size.
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
