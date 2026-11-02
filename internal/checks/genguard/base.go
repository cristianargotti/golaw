package genguard

import (
	"errors"
	"fmt"

	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/model"
)

// baseSnapshot is what the repository held before the candidate: the base
// of a hook overlay, or the snapshot itself when it keeps none, because a
// fixture tree is the base it stands for.
func baseSnapshot(in model.Input) model.Snapshot {
	if based, ok := in.Snapshot.(model.Based); ok {
		return based.Base()
	}
	return in.Snapshot
}

// baseContent reads path from the base; a path the base does not hold is
// new and answers false without an error.
func baseContent(in model.Input, path string) ([]byte, bool, error) {
	data, err := baseSnapshot(in).Read(path)
	if errors.Is(err, model.ErrNotInSnapshot) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read the base of %s: %w", path, err)
	}
	return data, true, nil
}

// manifestOf reads the manifest of s, or nil when it is absent, unreadable
// or does not decode: a broken manifest is the finding of golaw gen --check,
// and a guard that cannot read it must not deny (decision 10).
func manifestOf(s model.Snapshot) *manifest.Manifest {
	data, err := s.Read(manifest.Path)
	if err != nil {
		return nil
	}
	m, err := manifest.Decode(data)
	if err != nil {
		return nil
	}
	return &m
}
