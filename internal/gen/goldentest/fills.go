package goldentest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/imports"
	"github.com/cristianargotti/golaw/internal/source"
)

// fillMode is the mode of a filled owned file: the owner reads and writes
// it, as git checks it out.
const fillMode = 0o600

// recordedFill is one entry of a fills file as written; its import keys
// are spelled here because the kind contract's Import carries no tags.
type recordedFill struct {
	ID      string           `yaml:"id"`
	Body    string           `yaml:"body"`
	Imports []recordedImport `yaml:"imports"`
}

type recordedImport struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// fill answers the entry as the harness applies it.
func (r recordedFill) fill() Fill {
	f := Fill{ID: r.ID, Body: r.Body}
	for _, i := range r.Imports {
		f.Imports = append(f.Imports, genkind.Import{Name: i.Name, Path: i.Path})
	}
	return f
}

// LoadFills reads a kind's recorded fills, a top level sequence of Fill,
// refusing a record it cannot trust, so a typo never passes as a fill.
func LoadFills(path string) ([]Fill, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the fills path is the test's own
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFills, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var recorded []recordedFill
	if err := decoder.Decode(&recorded); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: %s: %w", ErrFills, path, err)
	}
	fills := make([]Fill, 0, len(recorded))
	seen := map[string]bool{}
	for _, r := range recorded {
		switch {
		case r.ID == "":
			return nil, fmt.Errorf("%w: %s: a fill has no id", ErrFills, path)
		case strings.TrimSpace(r.Body) == "":
			return nil, fmt.Errorf("%w: %s: fill %s has no body", ErrFills, path, r.ID)
		case seen[r.ID]:
			return nil, fmt.Errorf("%w: %s: duplicate fill %s", ErrFills, path, r.ID)
		}
		seen[r.ID] = true
		fills = append(fills, r.fill())
	}
	return fills, nil
}

// fill closes the holes the fills name, each in the file as it is now on
// disk, since one file may hold two holes; it answers false when a fill
// failed, because a later step would judge a slice the record does not
// describe.
func (h *harness) fill(ctx context.Context) bool {
	open, err := h.openHoles()
	if err != nil {
		h.failf(sliceWhere, "%v", err)
		return false
	}
	snap, err := snapshot(ctx, h.repo.Root)
	if err != nil {
		h.failf(sliceWhere, "%v", err)
		return false
	}
	index := imports.Std().With(h.s.Module, snap)
	ok := true
	for i := range h.s.Fills {
		f := &h.s.Fills[i]
		hole, found := open[f.ID]
		if !found {
			h.failf("fill "+f.ID, "names no open hole")
			ok = false
			continue
		}
		ok = h.fillOne(f, hole, index) && ok
	}
	return ok
}

// openHoles indexes by id the holes of the Go files the commands wrote.
func (h *harness) openHoles() (map[string]holes.Hole, error) {
	out := map[string]holes.Hole{}
	for _, path := range slices.Sorted(maps.Keys(h.report.Files)) {
		if !strings.HasSuffix(path, goSuffix) {
			continue
		}
		found, err := holes.Find(path, h.report.Files[path])
		if err != nil {
			return nil, fmt.Errorf("holes of %s: %w", path, err)
		}
		for _, hole := range found {
			out[hole.ID] = hole
		}
	}
	return out, nil
}

// fillOne writes one fill over the file that holds its hole.
func (h *harness) fillOne(f *Fill, hole holes.Hole, index imports.Index) bool {
	where := "fill " + f.ID
	file := source.OSPath(h.repo.Root, hole.Path)
	fix, err := filled(file, f, hole, index)
	if err != nil {
		h.failf(where, "%v", err)
		return false
	}
	if len(fix.Report) != 0 {
		h.failf(where, "imports left as they were: %s", strings.Join(fix.Report, "; "))
		return false
	}
	if err := os.WriteFile(file, fix.Content, fillMode); err != nil {
		h.failf(where, "write %s: %v", hole.Path, err)
		return false
	}
	return true
}

// filled replaces the body that holds the hole, adds the fill's imports
// and removes the ones the replaced body no longer uses.
func filled(file string, f *Fill, hole holes.Hole, index imports.Index) (imports.Fix, error) {
	content, err := os.ReadFile(file) //nolint:gosec // the path is one the commands wrote
	if err != nil {
		return imports.Fix{}, fmt.Errorf(readFailed, hole.Path, err)
	}
	line, err := currentLine(content, hole)
	if err != nil {
		return imports.Fix{}, err
	}
	out, err := replaceBody(hole.Path, content, line, f.Body)
	if err != nil {
		return imports.Fix{}, err
	}
	out, err = holes.Append(out, genkind.Append{Path: hole.Path, Imports: f.Imports})
	if err != nil {
		return imports.Fix{}, fmt.Errorf("add the imports: %w", err)
	}
	fix, err := imports.Apply(hole.Path, out, index)
	if err != nil {
		return imports.Fix{}, fmt.Errorf("fix the imports: %w", err)
	}
	// The hook formats every Go file an agent writes (M7.9.3 step 1), so a
	// recorded fill is judged in the shape the agent's file would land in.
	if fix.Content, err = format.Source(fix.Content); err != nil {
		return imports.Fix{}, fmt.Errorf("%w: format %s: %w", ErrFills, hole.Path, err)
	}
	return fix, nil
}

// currentLine answers where the hole sits in content now: an earlier fill
// of the same file changes its line count, so the line the commands left
// the hole on may point into another function or past the end.
func currentLine(content []byte, hole holes.Hole) (int, error) {
	found, err := holes.Find(hole.Path, content)
	if err != nil {
		return 0, fmt.Errorf("%w: holes of %s: %w", ErrFills, hole.Path, err)
	}
	for _, h := range found {
		if h.ID == hole.ID {
			return h.Line, nil
		}
	}
	return 0, fmt.Errorf("%w: %s no longer holds hole %s", ErrFills, hole.Path, hole.ID)
}

// replaceBody answers content with the statements of the function whose
// braces enclose line replaced by body.
func replaceBody(path string, content []byte, line int, body string) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, content, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrFills, path, err)
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		open, closing := fset.Position(fn.Body.Lbrace), fset.Position(fn.Body.Rbrace)
		if open.Line <= line && line <= closing.Line {
			out := slices.Concat(content[:open.Offset+1], []byte("\n"+strings.TrimRight(body, "\n")+"\n"), content[closing.Offset:])
			return out, nil
		}
	}
	return nil, fmt.Errorf("%w: %s:%d is in no function body", ErrFills, path, line)
}
