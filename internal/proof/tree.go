package proof

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// binaryProbe is how many leading bytes decide that a file is binary.
const binaryProbe = 8000

var generatedHeader = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

// Tree is an in-memory model.Snapshot built from a path to bytes map, for
// fixtures and unit tests. Files are sorted by path; classes follow the
// path and the content the way source classifies a worktree file.
type Tree struct {
	files map[string][]byte
	order []string
}

// NewTree copies the map into a frozen snapshot.
func NewTree(files map[string][]byte) *Tree {
	t := &Tree{files: make(map[string][]byte, len(files)), order: make([]string, 0, len(files))}
	for path, content := range files {
		t.files[path] = bytes.Clone(content)
		t.order = append(t.order, path)
	}
	slices.Sort(t.order)
	return t
}

// Files lists every path in order with its class and size.
func (t *Tree) Files() []model.File {
	out := make([]model.File, 0, len(t.order))
	for _, path := range t.order {
		out = append(out, model.File{Path: path, Class: t.Class(path), Size: int64(len(t.files[path]))})
	}
	return out
}

// Read returns a copy of the bytes, ErrNotInSnapshot for an unknown path
// and ErrTooLarge beyond the read bound.
func (t *Tree) Read(path string) ([]byte, error) {
	content, ok := t.files[path]
	if !ok {
		return nil, fmt.Errorf("%w: %s", model.ErrNotInSnapshot, path)
	}
	if len(content) > model.MaxReadBytes {
		return nil, fmt.Errorf("%w: %s", model.ErrTooLarge, path)
	}
	return bytes.Clone(content), nil
}

// Class classifies a path of the tree, ClassUnknown for an unknown one.
func (t *Tree) Class(path string) model.FileClass {
	content, ok := t.files[path]
	if !ok {
		return model.ClassUnknown
	}
	return Classify(path, content)
}

// Classify assigns the class a worktree file gets from its path and its
// content: binary before generated, generated before fixture, fixture
// before test, and tracked for everything else.
func Classify(path string, content []byte) model.FileClass {
	probe := content
	if len(probe) > binaryProbe {
		probe = probe[:binaryProbe]
	}
	switch {
	case bytes.IndexByte(probe, 0) >= 0:
		return model.ClassBinary
	case generatedHeader.Match(content):
		return model.ClassGenerated
	case strings.HasPrefix(path, "testdata/") || strings.Contains(path, "/testdata/"):
		return model.ClassFixture
	case strings.HasSuffix(path, "_test.go"):
		return model.ClassTest
	default:
		return model.ClassTracked
	}
}
