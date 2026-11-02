package gen

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

// maxNeedDepth bounds the chain of Needs one golaw new follows: a port needs
// its use case and nothing deeper exists, so a longer chain is a loop.
const maxNeedDepth = 3

// plan is the state of one run. It writes nothing: every write is pending
// until apply.Files lands it, so a failed run leaves the tree as it was.
type plan struct {
	in        Input
	manifest  manifest.Manifest
	writes    map[string]Write
	bindings  []genkind.Binding
	decisions []genkind.Decision
	touched   []manifest.Entry
	removed   []manifest.Entry
	owned     map[string]bool
	gone      []goneContract
	stubs     map[string]bool
	done      map[string]bool
}

// goneContract is a contract path a manifest entry names and the snapshot
// lacks, kept apart so the check reports it as drift with its own fix.
type goneContract struct {
	path  string
	entry manifest.Entry
}

func newPlan(in Input) *plan {
	return &plan{
		in: in, manifest: in.Manifest, writes: map[string]Write{},
		owned: map[string]bool{}, stubs: map[string]bool{}, done: map[string]bool{},
	}
}

// entryKey is the identity of an entry in the manifest: its kind and name.
func entryKey(kind, name string) string { return kind + " " + name }

// ref is the template value a kind of this kit writes in the manifest.
func ref(k genkind.Kind) string {
	return genheader.Header{Kind: k.Name(), Template: k.Template()}.Ref()
}

// input answers the kind input with the manifest and the files as the run
// left them, so a kind rendered after another sees the entry and the file
// the first one added.
func (p *plan) input() genkind.Input {
	in := p.in.Input
	in.Manifest = p.manifest
	in.Snapshot = p.overlay()
	return in
}

func (p *plan) kind(name string) (genkind.Kind, error) {
	if k, ok := p.lookupKind(name); ok {
		return k, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrUnknownKind, name)
}

// lookupKind answers the kind of that name when the kit holds it; the
// support and wiring kinds are optional, so their absence is no error.
func (p *plan) lookupKind(name string) (genkind.Kind, bool) {
	for _, k := range p.in.Kinds {
		if k.Name() == name {
			return k, true
		}
	}
	return nil, false
}

// wirer answers the wiring kind when the kit holds one; without it a run
// leaves the composition root alone.
func (p *plan) wirer() genkind.Wirer {
	k, ok := p.lookupKind(genkind.KindWiring)
	if !ok {
		return nil
	}
	w, ok := k.(genkind.Wirer)
	if !ok {
		return nil
	}
	return w
}

// snapshot answers the bytes the run read at path; a path the snapshot does
// not hold is absent, any other read failure stops the run.
func (p *plan) snapshot(path string) ([]byte, bool, error) {
	content, err := p.in.Snapshot.Read(path)
	if errors.Is(err, model.ErrNotInSnapshot) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", path, err)
	}
	return content, true, nil
}

// current answers the bytes the path will hold once the run lands: the
// pending write, else what the snapshot read.
func (p *plan) current(path string) ([]byte, bool, error) {
	if w, ok := p.writes[path]; ok {
		return w.Content, true, nil
	}
	return p.snapshot(path)
}

// put records a pending write bound to the bytes the snapshot read. A file
// the run creates and then appends to stays a creation, so apply.Files never
// treats it as an edit of a file that was not there.
func (p *plan) put(path string, content []byte, mode WriteMode) error {
	read, present, err := p.snapshot(path)
	if err != nil {
		return err
	}
	base := ""
	if present {
		base = source.ContentHash(read)
	}
	if old, ok := p.writes[path]; ok && old.Mode == WriteOwned {
		mode = WriteOwned
	}
	p.writes[path] = Write{Path: path, Content: content, Mode: mode, Base: base}
	return nil
}

// missingContract answers the first contract path of e the snapshot lacks, or
// the empty string when every contract is there.
func (p *plan) missingContract(e manifest.Entry) (string, error) {
	for _, c := range e.Contracts {
		_, present, err := p.snapshot(c.Path)
		if err != nil {
			return "", err
		}
		if !present {
			return c.Path, nil
		}
	}
	return "", nil
}

// putManifest records the manifest write when its encoding differs from the
// bytes read; a repository the generators never wrote into gets none, so
// golaw gen --check stays clean there.
func (p *plan) putManifest() error {
	if p.manifest.Support.Path == "" && len(p.manifest.Entries) == 0 {
		return nil
	}
	encoded, err := p.manifest.Encode()
	if err != nil {
		return fmt.Errorf("encode %s: %w", manifest.Path, err)
	}
	read, present, err := p.snapshot(manifest.Path)
	if err != nil || present && bytes.Equal(read, encoded) {
		return err
	}
	return p.put(manifest.Path, encoded, WriteManifest)
}
