package codex

import (
	"errors"
	"fmt"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// candidateOf reconstructs the candidate of one hunk with its bytes: the
// create or the replace of an Add File, the edit or the move of an Update
// File, and the deletion of a Delete File; every path is placed in the
// repository first, so a hunk outside it is named before any file is read.
func candidateOf(hunk Hunk, added []string, cwd string, base hook.Base, snapshot model.Snapshot) (model.Candidate, []byte, error) {
	path, err := patchPath(base, cwd, hunk.Path)
	if err != nil {
		return model.Candidate{}, nil, err
	}
	switch hunk.Op {
	case model.CandidateCreate, model.CandidateReplace:
		return addCandidate(path, hunk.Lines, snapshot)
	case model.CandidateDelete:
		return deleteCandidate(path, snapshot)
	case model.CandidateMove:
		to, toErr := patchPath(base, cwd, hunk.To)
		if toErr != nil {
			return model.Candidate{}, nil, toErr
		}
		return updateCandidate(path, to, hunk.Chunks, added, snapshot)
	case model.CandidateEdit:
		return updateCandidate(path, path, hunk.Chunks, added, snapshot)
	default:
		return model.Candidate{}, nil, fmt.Errorf("%w: unsupported patch operation %s", model.ErrCannotJudge, hunk.Op)
	}
}

// addCandidate is the whole content of an Add File, a line feed after each
// content line: a create of a path the snapshot does not hold, else a
// replace bound to the readable base. A base over the read bound binds no
// hash, because the whole content is judged and the freshness of the base
// is its cached stat identity, or its size when the snapshot has no stat.
func addCandidate(path string, lines []string, snapshot model.Snapshot) (model.Candidate, []byte, error) {
	candidate := model.Candidate{Path: path, Op: model.CandidateCreate, Reconstructed: true}
	if snapshot.Class(path) != model.ClassUnknown {
		candidate.Op = model.CandidateReplace
		base, err := snapshot.Read(path)
		switch {
		case err == nil:
			candidate.BaseHash = hook.Hash(base)
		case !errors.Is(err, model.ErrTooLarge):
			return model.Candidate{}, nil, fmt.Errorf("%w: apply_patch base %s: %w", model.ErrCannotJudge, path, err)
		}
	}
	return candidate, joinLines(lines, lineFeed), nil
}

// deleteCandidate is the deletion of a Delete File, whole because a
// deletion has no bytes to reconstruct, bound to the readable base it
// removes; a base over the read bound binds no hash, because a deletion
// has no bytes to judge.
func deleteCandidate(path string, snapshot model.Snapshot) (model.Candidate, []byte, error) {
	base, _, err := existing(path, snapshot)
	if err != nil {
		return model.Candidate{}, nil, err
	}
	candidate := model.Candidate{Path: path, Op: model.CandidateDelete, Reconstructed: true}
	if base != nil {
		candidate.BaseHash = hook.Hash(base)
	}
	return candidate, nil, nil
}

// updateCandidate is the edit of an Update File, or its move when the
// destination differs: the chunks applied to the base give the whole
// content and the hash of the base; chunks that do not apply give the
// added lines alone, which the core judges and names in its notice (D3);
// a base over the read bound is answered as too large with the candidate,
// so the caller leaves it to the names-only path.
func updateCandidate(from, to string, chunks []Chunk, added []string, snapshot model.Snapshot) (model.Candidate, []byte, error) {
	candidate := model.Candidate{Path: to, Op: model.CandidateEdit}
	if from != to {
		candidate.Op, candidate.From = model.CandidateMove, from
	}
	base, large, err := existing(from, snapshot)
	if err != nil {
		return model.Candidate{}, nil, err
	}
	if large {
		return candidate, nil, fmt.Errorf("%w: apply_patch base %s", model.ErrTooLarge, from)
	}
	candidate.BaseHash = hook.Hash(base)
	if content, applied := ApplyChunks(base, chunks); applied {
		candidate.Reconstructed = true
		return candidate, content, nil
	}
	return candidate, joinLines(added, lineFeed), nil
}

// existing is the content of a file a hunk updates, moves or deletes, or
// none with large true when it is over the read bound. A path the snapshot
// does not hold is cannot judge with the fix named, because Codex cannot
// perform these operations on an absent source and the agent must correct
// its patch; any other unreadable base is cannot judge as well.
func existing(path string, snapshot model.Snapshot) ([]byte, bool, error) {
	if snapshot.Class(path) == model.ClassUnknown {
		return nil, false, absentSource(path)
	}
	base, err := snapshot.Read(path)
	switch {
	case err == nil:
		return base, false, nil
	case errors.Is(err, model.ErrTooLarge):
		return nil, true, nil
	default:
		return nil, false, fmt.Errorf("%w: apply_patch base %s: %w", model.ErrCannotJudge, path, err)
	}
}

// absentSource is the answer to a hunk whose source the snapshot does not
// hold, with the fix the agent can apply.
func absentSource(path string) error {
	return fmt.Errorf("%w: %s does not exist, and Codex cannot update, move or delete an absent file: correct the path in the patch", model.ErrCannotJudge, path)
}
