package gen

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/genheader"
)

// stubPackage wraps one declaration so its key reads alone.
const stubPackage = "package p\n\n"

// apply turns one kind output into pending writes: regenerable files that
// differ, owned files that do not exist yet and the stubs owned files lack.
func (p *plan) apply(out genkind.Output) error {
	if err := p.applyRegenerable(out.Regenerable); err != nil {
		return err
	}
	for _, f := range out.Owned {
		p.owned[f.Path] = true
		_, present, err := p.current(f.Path)
		if err != nil {
			return err
		}
		if !present {
			if err := p.put(f.Path, f.Content, WriteOwned); err != nil {
				return err
			}
		}
	}
	for _, a := range out.Appends {
		if err := p.appendStubs(a); err != nil {
			return err
		}
	}
	p.bindings = append(p.bindings, out.Bindings...)
	p.decisions = append(p.decisions, out.Decisions...)
	for _, path := range out.Entry.Owned {
		p.owned[path] = true
	}
	return nil
}

// applyRegenerable writes each rendering whose bytes differ from the file's
// but for the version token; a handwritten file is never generated into
// (M7.9.5), whatever the manifest says.
func (p *plan) applyRegenerable(files []genkind.File) error {
	for _, f := range files {
		read, present, err := p.snapshot(f.Path)
		if err != nil {
			return err
		}
		if present && (handwritten(read) || genheader.SameIgnoringVersion(read, f.Content)) {
			continue
		}
		if err := p.put(f.Path, f.Content, WriteRegenerable); err != nil {
			return err
		}
	}
	return nil
}

func handwritten(content []byte) bool {
	_, marked := genheader.Handwritten(content)
	return marked
}

// appendStubs appends the declarations an owned file lacks. A declaration
// the file already holds is the agent's and stays as written (A40), and an
// owned file that is gone is a decision, never recreated behind its back.
func (p *plan) appendStubs(a genkind.Append) error {
	content, present, err := p.current(a.Path)
	if err != nil {
		return err
	}
	if !present {
		p.decisions = append(p.decisions, genkind.Decision{Path: a.Path, Text: fmt.Sprintf(
			"%s is gone: the stubs of %s were not appended; restore it or remove the entry", a.Path, strings.Join(a.Holes, ", "))})
		return nil
	}
	if handwritten(content) {
		return nil
	}
	missing, err := missingDecls(a.Path, content, a.Decls)
	if err != nil || len(missing) == 0 {
		return err
	}
	a.Decls = missing
	appended, err := holes.Append(content, a)
	if err != nil {
		return fmt.Errorf("append to %s: %w", a.Path, err)
	}
	p.stubs[a.Path] = true
	return p.put(a.Path, appended, WriteAppend)
}

// missingDecls answers the declarations whose key the file does not declare.
func missingDecls(rel string, content []byte, decls [][]byte) ([][]byte, error) {
	declared, err := holes.Names(content)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	var out [][]byte
	for _, decl := range decls {
		keys, err := holes.Names(append([]byte(stubPackage), decl...))
		if err != nil {
			return nil, fmt.Errorf("read a stub: %w", err)
		}
		if len(keys) == 0 || !slices.Contains(declared, keys[0]) {
			out = append(out, decl)
		}
	}
	return out, nil
}
