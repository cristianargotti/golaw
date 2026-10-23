package codex

import (
	"errors"
	"fmt"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// heredocCat is the request of a cat here-document: the shell event with
// the file it writes as its write. A target outside the repository the
// event runs in, such as the scratch script an agent writes before it runs
// it, is no write of this repository: the event keeps the shell route, and
// the detection of its post event judges what the command wrote inside
// (Review Focus 1 of the plan).
func heredocCat(request hook.Request, form heredoc, base hook.Base, snapshot model.Snapshot) (hook.Request, error) {
	path, inside := catPath(base, request.Event.Cwd, form.target)
	if !inside {
		return request, nil
	}
	candidate, content, err := catCandidate(path, form, snapshot)
	if err != nil {
		return hook.Request{}, err
	}
	request.Writes = &hook.Writes{
		Candidates: []model.Candidate{candidate},
		Content:    map[string][]byte{path: content},
	}
	return request, nil
}

// catPath is the target of a cat from the root of the repository, and
// whether the repository holds it.
func catPath(base hook.Base, cwd, target string) (string, bool) {
	path, err := base.Relative(cwd, target)
	return path, err == nil
}

// catCandidate is the candidate of the file a cat writes, with its bytes.
// A replacement is the whole new file, bound to the readable base it
// replaces. An append to an absent file is the whole new file as well, the
// appended bytes; an append to a readable file is its bytes followed by
// the body, exactly what the shell leaves, bound to that base as an edit
// of Claude is, since the recognizer accepts only a body the shell writes
// as it reads. An append to a base over the read bound holds only the
// appended bytes, not reconstructed, so the core judges the added lines
// and names what it could not measure (D9); such a base binds no hash, as
// a patch's does.
func catCandidate(path string, form heredoc, snapshot model.Snapshot) (model.Candidate, []byte, error) {
	body := []byte(form.body)
	candidate := model.Candidate{Path: path, Op: model.CandidateCreate, Reconstructed: true}
	if snapshot.Class(path) == model.ClassUnknown {
		return candidate, body, nil
	}
	candidate.Op = model.CandidateReplace
	if form.appends {
		candidate.Op = model.CandidateEdit
	}
	base, err := snapshot.Read(path)
	switch {
	case err == nil && form.appends:
		candidate.BaseHash = hook.Hash(base)
		return candidate, append(append(make([]byte, 0, len(base)+len(body)), base...), body...), nil
	case err == nil:
		candidate.BaseHash = hook.Hash(base)
	case errors.Is(err, model.ErrTooLarge):
		candidate.Reconstructed = !form.appends
	default:
		return model.Candidate{}, nil, fmt.Errorf("%w: here-document target %s: %w", model.ErrCannotJudge, path, err)
	}
	return candidate, body, nil
}
