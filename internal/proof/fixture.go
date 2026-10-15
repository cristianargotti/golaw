package proof

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// The two conventions of a fixture tree.
const (
	// EventFile is the normalized event an event check fixture carries.
	EventFile = "event.json"
	// EscapedSuffix marks a file whose content holds Go unicode escapes.
	EscapedSuffix = ".esc"
)

// Fixture is one loaded red or near-miss tree: its files at their in-tree
// paths and, for an event check, the normalized event.
type Fixture struct {
	Files map[string][]byte
	Event *model.Event
}

// LoadFixture reads a fixture directory. A .esc file is decoded and loses
// its suffix, event.json becomes the Event, and an empty tree is an error
// because it would prove nothing.
func LoadFixture(dir string) (Fixture, error) {
	f := Fixture{Files: make(map[string][]byte)}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("relative path of %s: %w", path, err)
		}
		content, err := os.ReadFile(path) //nolint:gosec // the fixture path is the test's own
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		return f.add(filepath.ToSlash(rel), content)
	})
	if err != nil {
		return Fixture{}, fmt.Errorf("%w: %s: %w", ErrFixture, dir, err)
	}
	if len(f.Files) == 0 && f.Event == nil {
		return Fixture{}, fmt.Errorf("%w: %s holds no file and no event", ErrFixture, dir)
	}
	return f, nil
}

// escapeWidth is the length of a \uXXXX sequence.
const escapeWidth = 6

// Unescape decodes \uXXXX and \\ sequences and leaves every other byte as
// it is, so a fixture spells a prohibited rune without carrying it.
func Unescape(text []byte) []byte {
	out := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		if text[i] != '\\' || i+1 >= len(text) {
			out = append(out, text[i])
			continue
		}
		if text[i+1] == '\\' {
			out = append(out, '\\')
			i++
			continue
		}
		if decoded, ok := unicodeEscape(text[i:]); ok {
			out = append(out, decoded...)
			i += escapeWidth - 1
			continue
		}
		out = append(out, text[i])
	}
	return out
}

// unicodeEscape decodes a \uXXXX sequence at the start of text through the
// Go string grammar, so an invalid sequence stays literal.
func unicodeEscape(text []byte) (string, bool) {
	if len(text) < escapeWidth || text[1] != 'u' {
		return "", false
	}
	decoded, err := strconv.Unquote(`"` + string(text[:escapeWidth]) + `"`)
	if err != nil {
		return "", false
	}
	return decoded, true
}

// Policy is the policy every fixture runs under: every capability enabled,
// the core floors, enrolled, so a check conditioned by a capability still
// reaches its fixture.
func Policy() *model.EffectivePolicy {
	p := model.CoreDefaults()
	p.Digest = "proof"
	p.Capabilities = model.Capabilities()
	p.Enrollment = model.Enrollment{State: model.EnrollmentEnrolled, PolicyDigest: "proof"}
	return &p
}

// Input builds the dispatch input of the fixture for a check scope: a scan
// over the tree for file and tree checks, the commit rung for commit
// checks, a hook with the event for event checks.
func (f *Fixture) Input(scope model.Scope, policy *model.EffectivePolicy) (model.Input, error) {
	tree := NewTree(f.Files)
	in := model.Input{Policy: policy, Snapshot: tree}
	switch scope {
	case model.ScopeFile, model.ScopeTree:
		in.Mode, in.Targets = model.ModeScan, tree.Files()
	case model.ScopeCommit:
		in.Mode, in.Targets = model.ModeCommit, tree.Files()
	case model.ScopeEvent:
		if f.Event == nil {
			return model.Input{}, fmt.Errorf("%w: an event check fixture needs %s", ErrFixture, EventFile)
		}
		in.Mode, in.Event, in.Targets = model.ModeHook, f.Event, eventTargets(f.Event, tree)
	default:
		return model.Input{}, fmt.Errorf("%w: scope %s", ErrFixture, scope)
	}
	return in, nil
}

// eventTargets lists the candidate or changed files the tree holds.
func eventTargets(e *model.Event, tree *Tree) []model.File {
	var paths []string
	switch e.Shape {
	case model.ShapeWrite:
		for _, c := range e.Candidates {
			paths = append(paths, c.Path)
		}
	case model.ShapePost:
		paths = e.Post.Changed
	case model.ShapeRead, model.ShapeShell:
		return nil
	default:
		return nil
	}
	var out []model.File
	for _, path := range paths {
		if class := tree.Class(path); class != model.ClassUnknown {
			out = append(out, model.File{Path: path, Class: class, Size: int64(len(tree.files[path]))})
		}
	}
	return out
}

func (f *Fixture) add(rel string, content []byte) error {
	if rel == EventFile {
		var e model.Event
		if err := json.Unmarshal(content, &e); err != nil {
			return fmt.Errorf("%s: %w", EventFile, err)
		}
		if err := e.Validate(); err != nil {
			return fmt.Errorf("%s: %w", EventFile, err)
		}
		f.Event = &e
		return nil
	}
	if plain, ok := strings.CutSuffix(rel, EscapedSuffix); ok {
		rel, content = plain, Unescape(content)
	}
	f.Files[rel] = content
	return nil
}
