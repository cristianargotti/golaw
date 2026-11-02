package gentest

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

// baseMessage is the subject of the fixture's base commit, conventional so a
// commit check over the repository never reports the fixture itself.
const baseMessage = "chore: base of the generator fixture"

// Module writes files into a new git repository under the test's temporary
// directory, commits them as the base and answers the repository, so the
// coverage on new code and the hook compare against a real base commit.
func Module(tb testing.TB, files map[string][]byte) *gittest.Repo {
	tb.Helper()
	text := make(map[string]string, len(files))
	for path, content := range files {
		text[path] = string(content)
	}
	repo := gittest.New(tb)
	repo.Commit(baseMessage, text)
	return repo
}

// Snapshot is an in-memory model.Snapshot over a copy of files, every file
// tracked with the class source.Classify gives it, so a pure kind or engine
// test sees the classes a worktree snapshot would give the same bytes.
func Snapshot(tb testing.TB, files map[string][]byte) model.Snapshot {
	tb.Helper()
	m := memory{content: make(map[string][]byte, len(files))}
	for _, path := range slices.Sorted(maps.Keys(files)) {
		content := bytes.Clone(files[path])
		m.content[path] = content
		m.files = append(m.files, model.File{
			Path: path, Class: source.Classify(path, content, true), Size: int64(len(content)),
		})
	}
	return m
}

// memory is the snapshot Snapshot answers; it is read only after it is built,
// so tests running in parallel share it without a lock.
type memory struct {
	files   []model.File
	content map[string][]byte
}

// Files answers the files sorted by path.
func (m memory) Files() []model.File { return slices.Clone(m.files) }

// Read answers a copy, so a caller that edits it never changes the snapshot.
func (m memory) Read(path string) ([]byte, error) {
	content, ok := m.content[path]
	if !ok {
		return nil, fmt.Errorf("%w: %s", model.ErrNotInSnapshot, path)
	}
	return bytes.Clone(content), nil
}

// Class answers model.ClassUnknown for a path the snapshot does not hold.
func (m memory) Class(path string) model.FileClass {
	i, ok := slices.BinarySearchFunc(m.files, path, func(f model.File, p string) int {
		return strings.Compare(f.Path, p)
	})
	if !ok {
		return model.ClassUnknown
	}
	return m.files[i].Class
}
