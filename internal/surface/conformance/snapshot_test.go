package conformance_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/surfacefix"
)

// twinSnapshot is a twin of the fixture repository as a frozen snapshot:
// every file tracked, read from the decoded tree.
type twinSnapshot map[string][]byte

func twin(t *testing.T, which surfacefix.Twin) twinSnapshot {
	t.Helper()
	files, err := surfacefix.Files(which)
	if err != nil {
		t.Fatal(err)
	}
	return twinSnapshot(files)
}

func (s twinSnapshot) Files() []model.File {
	out := make([]model.File, 0, len(s))
	for _, name := range slices.Sorted(maps.Keys(s)) {
		out = append(out, model.File{Path: name, Class: model.ClassTracked, Size: int64(len(s[name]))})
	}
	return out
}

func (s twinSnapshot) Read(name string) ([]byte, error) {
	content, ok := s[name]
	if !ok {
		return nil, model.ErrNotInSnapshot
	}
	return content, nil
}

func (s twinSnapshot) Class(name string) model.FileClass {
	if _, ok := s[name]; ok {
		return model.ClassTracked
	}
	return model.ClassUnknown
}
