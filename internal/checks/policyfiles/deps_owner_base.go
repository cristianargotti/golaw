package policyfiles

import (
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

// baseOwnership preserves base file precedence so staged edits cannot change authority.
func baseOwnership(in model.Input) (string, owners, error) {
	for _, path := range []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"} {
		data, found, err := baseOwnerBytes(in, path)
		if err != nil {
			return path, owners{}, err
		}
		if found {
			rules, parseErr := readOwners(path, data)
			return path, rules, parseErr
		}
	}
	return "", owners{}, nil
}

// baseOwnerBytes recovers pre-change content so staged ownership edits cannot authorize themselves.
func baseOwnerBytes(in model.Input, path string) ([]byte, bool, error) {
	before, existed, introduced := ownerBaseChange(in.Changes, path)
	if existed {
		data, err := model.ReadBlob(in.Snapshot, before)
		if err != nil {
			return nil, true, fmt.Errorf("read staged base of %s: %w", path, err)
		}
		return data, true, nil
	}
	if introduced {
		return nil, false, nil
	}
	data, found, err := read(in, path)
	if err != nil {
		return nil, found, fmt.Errorf("read staged base of %s: %w", path, err)
	}
	return data, found, nil
}

// An outgoing path existed before any incoming replacement, regardless of
// the order in which the staged changes are listed.
func ownerBaseChange(changes []model.Change, path string) (string, bool, bool) {
	var before string
	var existed, introduced bool
	for _, change := range changes {
		switch change.Op {
		case model.ChangeModify, model.ChangeDelete, model.ChangeRename:
			if change.OldPath == path {
				before, existed = change.Before, true
			}
		case model.ChangeAdd:
		}
		if (change.Op == model.ChangeAdd || change.Op == model.ChangeRename) && change.NewPath == path {
			introduced = true
		}
	}
	return before, existed, introduced
}
