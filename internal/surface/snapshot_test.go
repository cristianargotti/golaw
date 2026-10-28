package surface_test

import (
	"maps"
	"slices"
	"sync"

	"github.com/cristianargotti/golaw/internal/model"
)

// memSnapshot is a frozen snapshot over a map: every file tracked unless a
// class overrides it, a read error per path, and the paths it was asked to
// read, so a test proves what Take reads.
type memSnapshot struct {
	files   map[string][]byte
	classes map[string]model.FileClass
	failing map[string]error

	mu    sync.Mutex
	reads []string
}

func snapshotOf(files map[string]string) *memSnapshot {
	m := &memSnapshot{files: map[string][]byte{}, classes: map[string]model.FileClass{}, failing: map[string]error{}}
	for name, content := range files {
		m.files[name] = []byte(content)
	}
	return m
}

func (m *memSnapshot) Files() []model.File {
	out := make([]model.File, 0, len(m.files))
	for _, name := range slices.Sorted(maps.Keys(m.files)) {
		out = append(out, model.File{Path: name, Class: m.Class(name), Size: int64(len(m.files[name]))})
	}
	return out
}

func (m *memSnapshot) Read(name string) ([]byte, error) {
	m.mu.Lock()
	m.reads = append(m.reads, name)
	m.mu.Unlock()
	if err := m.failing[name]; err != nil {
		return nil, err
	}
	content, ok := m.files[name]
	if !ok {
		return nil, model.ErrNotInSnapshot
	}
	return content, nil
}

func (m *memSnapshot) Class(name string) model.FileClass {
	if class, ok := m.classes[name]; ok {
		return class
	}
	if _, ok := m.files[name]; ok {
		return model.ClassTracked
	}
	return model.ClassUnknown
}

// read answers the paths the snapshot was asked to read, in order.
func (m *memSnapshot) read() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.reads)
}
