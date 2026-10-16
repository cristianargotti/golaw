package claude

import (
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

type edit struct {
	Old *string `json:"old_string"`
	New *string `json:"new_string"`
	All bool    `json:"replace_all"`
}

type editInput struct {
	Path  string  `json:"file_path"`
	Old   *string `json:"old_string"`
	New   *string `json:"new_string"`
	All   bool    `json:"replace_all"`
	Edits []edit  `json:"edits"`
}

func editCandidate(w wire, at hook.Base, snapshot model.Snapshot) (model.Candidate, []byte, error) {
	var input editInput
	if err := strictInput(w.Input, &input); err != nil {
		return model.Candidate{}, nil, err
	}
	path, err := at.Relative(w.Cwd, input.Path)
	if err != nil {
		return model.Candidate{}, nil, fmt.Errorf("%w: Edit path: %w", model.ErrCannotJudge, err)
	}
	base, err := snapshot.Read(path)
	if err != nil {
		return model.Candidate{Path: path, Op: model.CandidateEdit}, nil, fmt.Errorf("%w: Edit base: %w", model.ErrCannotJudge, err)
	}
	edits, err := editSequence(input)
	if err != nil {
		return model.Candidate{}, nil, err
	}
	content, err := applyEdits(string(base), edits)
	if err != nil {
		return model.Candidate{}, nil, err
	}
	return model.Candidate{
		Path: path, Op: model.CandidateEdit,
		Reconstructed: true, BaseHash: hook.Hash(base),
	}, []byte(content), nil
}

func editSequence(input editInput) ([]edit, error) {
	if len(input.Edits) > 0 {
		if input.Old != nil || input.New != nil || input.All {
			return nil, fmt.Errorf("%w: mixed Edit schemas", model.ErrCannotJudge)
		}
		return input.Edits, nil
	}
	return []edit{{Old: input.Old, New: input.New, All: input.All}}, nil
}

func applyEdits(content string, edits []edit) (string, error) {
	for _, change := range edits {
		if change.Old == nil || change.New == nil || *change.Old == "" {
			return "", fmt.Errorf("%w: incomplete Edit operation", model.ErrCannotJudge)
		}
		count := strings.Count(content, *change.Old)
		if count == 0 || count > 1 && !change.All {
			return "", fmt.Errorf("%w: Edit target is absent or ambiguous", model.ErrCannotJudge)
		}
		content = strings.ReplaceAll(content, *change.Old, *change.New)
	}
	return content, nil
}
