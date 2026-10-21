package proof

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
)

// StagedFile is metadata so base blobs never become measured files.
const StagedFile = "staged.json"

// StagedFixture preserves the base evidence needed to judge a staged tree.
type StagedFixture struct {
	Changes []model.Change    `json:"changes"`
	Blobs   map[string]string `json:"blobs"`
}

func (f *Fixture) addStaged(content []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var staged *StagedFixture
	if err := decoder.Decode(&staged); err != nil { //nolint:musttag // model.Change's existing field names define the fixture JSON format.
		return fmt.Errorf("decode the staged changes: %w", err)
	}
	var rest json.RawMessage
	if err := decoder.Decode(&rest); !errors.Is(err, io.EOF) {
		return errStagedTrailing
	}
	if staged == nil {
		return errStagedObject
	}
	for i, change := range staged.Changes {
		if !validStagedChange(change) {
			return fmt.Errorf("%w at index %d (operation %s, %d)", errStagedChange, i, change.Op, change.Op)
		}
	}
	f.Staged = staged
	return nil
}

func validStagedChange(change model.Change) bool {
	switch change.Op {
	case model.ChangeAdd:
		return change.OldPath == "" && change.NewPath != "" && change.Before == ""
	case model.ChangeModify:
		return change.OldPath != "" && change.OldPath == change.NewPath
	case model.ChangeDelete:
		return change.OldPath != "" && change.NewPath == "" && change.After == ""
	case model.ChangeRename:
		return change.OldPath != "" && change.NewPath != "" && change.OldPath != change.NewPath
	default:
		return false
	}
}

func (f *Fixture) stagedInput(in model.Input) model.Input {
	if f.Staged == nil {
		return in
	}
	blobs := make(map[string][]byte, len(f.Staged.Blobs))
	for id, content := range f.Staged.Blobs {
		blobs[id] = []byte(content)
	}
	in.Mode, in.Changes = model.ModeStaged, slices.Clone(f.Staged.Changes)
	in.Snapshot = NewBlobTree(f.Files, blobs)
	return in
}
