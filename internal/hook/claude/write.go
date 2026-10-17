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
	w, event, in, err := parse(data)
	if err != nil {
		return hook.Request{}, err
	}
	return writeRequest(w, event, in, base, snapshot)
}

// writeRequest is the request of a pre event of a write tool from its one
// parse: the candidate with its bytes, the compatibility denial of
// NotebookEdit, or the gap of an edit of a file over the read bound.
func writeRequest(w wire, event model.Event, in toolInput, base hook.Base, snapshot model.Snapshot) (hook.Request, error) {
	if w.Name != preEvent {
		return hook.Request{}, fmt.Errorf("%w: expected pre event", model.ErrCannotJudge)
	}
	if event.Tool == notebookTool {
		return hook.Request{Event: event, Denial: notebookDenial}, nil
	}
	candidate, content, err := candidateOf(w, in, base, snapshot)
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
func candidateOf(w wire, in toolInput, base hook.Base, snapshot model.Snapshot) (model.Candidate, []byte, error) {
	switch w.Tool {
	case writeTool:
		return fullCandidate(w, in, base, snapshot)
	case editTool:
		return editCandidate(w, in, base, snapshot)
	default:
		return model.Candidate{}, nil, fmt.Errorf("%w: unsupported write tool", model.ErrCannotJudge)
	}
}

func fullCandidate(w wire, in toolInput, base hook.Base, snapshot model.Snapshot) (model.Candidate, []byte, error) {
	if in.err != nil {
		return model.Candidate{}, nil, in.err
	}
	input := in.write
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
		// judged, and the freshness of the base is its cached stat identity,
		// or its size when the snapshot has no stat.
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
