package claude

import (
	"fmt"
	"io"
	"strings"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// notebookDenial is the compatibility refusal of NotebookEdit, whatever
// the size of its event.
const notebookDenial = "NotebookEdit is not supported by this adapter"

// Oversized parses an event whose input is over the bound of the hook from
// the whole input, in bounded memory: the copy the shrink answers is parsed
// as any event, so an input the decoders refuse within the bound is refused
// over it, and a field the hook reads that the copy elided refuses the
// event. Its decoder answers what an event over the bound can be: a write
// is the candidate of its path with the gap of the input, NotebookEdit the
// compatibility denial, a post event the request of any post event, and any
// other event cannot be judged.
func Oversized(input io.Reader) (hook.Parsed, error) {
	shrunk, err := shrink(input)
	if err != nil {
		return hook.Parsed{}, fmt.Errorf("an input over the bound: %w", err)
	}
	w, event, in, err := parse(shrunk)
	if err != nil {
		return hook.Parsed{}, err
	}
	for _, field := range []string{w.Name, w.Tool, w.Cwd, w.Session, w.Agent, w.Use, in.target} {
		if strings.ContainsRune(field, 0) {
			return hook.Parsed{}, fmt.Errorf("%w: a field of an input over the bound is longer than the hook keeps", model.ErrCannotJudge)
		}
	}
	return hook.Parsed{Event: event, Decode: func(base hook.Base, snapshot model.Snapshot) (hook.Request, error) {
		return oversizedRequest(w, event, in, base, snapshot)
	}}, nil
}

// oversizedRequest is the request of an event over the bound.
func oversizedRequest(w wire, event model.Event, in toolInput, base hook.Base, snapshot model.Snapshot) (hook.Request, error) {
	switch {
	case event.Shape == model.ShapePost:
		return postRequest(w, event, in, base)
	case event.Tool == notebookTool:
		return hook.Request{Event: event, Denial: notebookDenial}, nil
	case event.Tool == writeTool || event.Tool == editTool:
		return unheldWrite(w, event, in, base, snapshot)
	default:
		return hook.Request{}, fmt.Errorf("%w: %w: the event is over the 8 MiB the hook reads", model.ErrCannotJudge, model.ErrTooLarge)
	}
}

// unheldWrite is a write over the bound checked as the decoder checks one
// within it, but for the bytes it does not hold: the candidate of its path,
// with the gap of the input in place of its content.
func unheldWrite(w wire, event model.Event, in toolInput, base hook.Base, snapshot model.Snapshot) (hook.Request, error) {
	if in.err != nil {
		return hook.Request{}, in.err
	}
	op := model.CandidateEdit
	switch {
	case event.Tool == editTool:
		if err := checkEdits(in.edit); err != nil {
			return hook.Request{}, err
		}
	case in.write.Content == nil:
		return hook.Request{}, fmt.Errorf("%w: Write content is absent", model.ErrCannotJudge)
	default:
		op = model.CandidateCreate
	}
	path, err := base.Relative(w.Cwd, in.target)
	if err != nil {
		return hook.Request{}, fmt.Errorf("%w: %s path: %w", model.ErrCannotJudge, event.Tool, err)
	}
	if op == model.CandidateCreate && snapshot.Class(path) != model.ClassUnknown {
		op = model.CandidateReplace
	}
	event.Shape = model.ShapeWrite
	event.Candidates = []model.Candidate{{Path: path, Op: op}}
	return hook.Request{Event: event, Gaps: []hook.Gap{hook.OverInputBound(path)}}, nil
}
