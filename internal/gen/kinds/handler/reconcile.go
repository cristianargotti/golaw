package handler

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/model"
)

// reconciled is what one regeneration asks of the owned part of a handler:
// new owned files at absent paths, appends at the end of existing ones and
// decisions for the person, never a rewrite of an owned byte (M7.4.2 rule 5).
type reconciled struct {
	Owned     []genkind.File
	Appends   []genkind.Append
	Decisions []genkind.Decision
	Paths     []string
}

// reconcile compares the operations of the document with the methods the
// owned files of the entry declare: a new operation gets its stub and red
// test, a removed one a decision, and Paths is the entry's new file list.
func reconcile(v view, s model.Snapshot, ownedPaths []string) (reconciled, error) {
	p, err := scan(s, ownedPaths)
	if err != nil {
		return reconciled{}, err
	}
	var missing []operation
	for _, op := range v.Ops {
		if _, ok := p.Methods[op.Name]; !ok {
			missing = append(missing, op)
		}
	}
	var r reconciled
	for _, g := range groups(missing) {
		if err := place(v, s, ownedPaths, p, g, &r); err != nil {
			return reconciled{}, err
		}
	}
	r.Decisions = append(r.Decisions, removed(v, p)...)
	slices.SortStableFunc(r.Decisions, func(a, b genkind.Decision) int {
		return cmp.Or(strings.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line))
	})
	r.Appends = merged(r.Appends)
	slices.SortFunc(r.Owned, func(a, b genkind.File) int { return strings.Compare(a.Path, b.Path) })
	r.Paths = slices.Clone(ownedPaths)
	for _, f := range r.Owned {
		r.Paths = append(r.Paths, f.Path)
	}
	slices.Sort(r.Paths)
	r.Paths = slices.Compact(r.Paths)
	return r, nil
}

// place puts the missing operations of one stem into the last chunk the
// entry owns while it has room, then into new chunks after it.
func place(v view, s model.Snapshot, ownedPaths []string, p present, g group, r *reconciled) error {
	last := lastChunk(v.Dir, g.Stem, ownedPaths)
	rest := g.Ops
	if last > 0 {
		var err error
		if rest, err = fill(v, p, g.Stem, last, rest, r); err != nil {
			return err
		}
	}
	return open(v, s, ownedPaths, g.Stem, last+1, rest, r)
}

// lastChunk is the highest n whose code path the entry owns, 0 for none.
func lastChunk(dir, stem string, ownedPaths []string) int {
	last := 0
	for n := 1; n <= len(ownedPaths); n++ {
		if code, _ := chunkPaths(dir, stem, n); slices.Contains(ownedPaths, code) {
			last = n
		}
	}
	return last
}

// fill appends to chunk n as many operations as it has room for and answers
// the rest; a deleted owned file of the chunk is written again holding only
// the new operations, because there is no byte of the agent's left to keep.
func fill(v view, p present, stem string, n int, ops []operation, r *reconciled) ([]operation, error) {
	code, test := chunkPaths(v.Dir, stem, n)
	fit := min(max(0, opsPerFile-p.Count[code]), len(ops))
	var newCode, newTests []operation
	for _, op := range ops[:fit] {
		method, red, err := stub(v, op)
		if err != nil {
			return nil, err
		}
		if p.Files[code] {
			r.Appends = append(r.Appends, genkind.Append{Path: code, Decls: [][]byte{method}, Imports: codeImports(v), Holes: []string{op.Hole}})
		} else {
			newCode = append(newCode, op)
		}
		switch {
		case !p.Files[test]:
			newTests = append(newTests, op)
		case !p.Tests["Test"+op.Name+"HoleIsOpen"]:
			r.Appends = append(r.Appends, genkind.Append{Path: test, Decls: [][]byte{red}, Imports: testImports(v)})
		}
	}
	var specs []spec
	if len(newCode) > 0 {
		specs = append(specs, codeSpec(v, code, newCode))
	}
	if len(newTests) > 0 {
		specs = append(specs, testSpec(v, test, newTests))
	}
	return ops[fit:], write(v, specs, r)
}

// open writes new chunks from n on, opsPerFile operations each; a chunk
// whose path holds a file outside the entry is a decision and no write,
// because golaw never writes a path outside the manifest's entries (M7.9.1).
func open(v view, s model.Snapshot, ownedPaths []string, stem string, n int, ops []operation, r *reconciled) error {
	for i, part := range chunk(ops, opsPerFile) {
		code, test := chunkPaths(v.Dir, stem, n+i)
		if taken := outside(s, ownedPaths, code, test); taken != "" {
			for _, op := range part {
				r.Decisions = append(r.Decisions, genkind.Decision{Path: taken, Line: 1, Text: fmt.Sprintf(
					"(*Handler).%s is missing: %s exists outside the manifest entry of handler %s, so add the method there or move that file",
					op.Name, taken, v.Package)})
			}
			continue
		}
		if err := write(v, operationSpecs(v, stem, n+i, part), r); err != nil {
			return err
		}
	}
	return nil
}

// outside answers the first path the snapshot holds that the entry does not
// own, so a hand written file of the same name is never overwritten.
func outside(s model.Snapshot, ownedPaths []string, paths ...string) string {
	for _, file := range paths {
		if s.Class(file) != model.ClassUnknown && !slices.Contains(ownedPaths, file) {
			return file
		}
	}
	return ""
}

func write(v view, specs []spec, r *reconciled) error {
	if len(specs) == 0 {
		return nil
	}
	files, err := renderAll(v.Module, specs, genheader.Header{})
	if err != nil {
		return err
	}
	r.Owned = append(r.Owned, files...)
	return nil
}

// removed is a decision per operation method the document no longer
// declares: deleting the agent's code is the person's call, never golaw's.
func removed(v view, p present) []genkind.Decision {
	ops := make(map[string]bool, len(v.Ops))
	for _, op := range v.Ops {
		ops[op.Name] = true
	}
	var out []genkind.Decision
	for _, m := range p.Methods {
		if !ops[m.Name] {
			out = append(out, genkind.Decision{
				Path: m.Path, Line: m.Line,
				Text: fmt.Sprintf("(*Handler).%s is no longer in %s: delete it or restore the operation in %s", m.Name, v.API, v.API),
			})
		}
	}
	return out
}

// merged folds the appends of one path into one, the declarations in
// operation order, and sorts them by path.
func merged(appends []genkind.Append) []genkind.Append {
	var out []genkind.Append
	at := map[string]int{}
	for _, a := range appends {
		i, ok := at[a.Path]
		if !ok {
			at[a.Path] = len(out)
			out = append(out, a)
			continue
		}
		out[i].Decls = append(out[i].Decls, a.Decls...)
		out[i].Holes = append(out[i].Holes, a.Holes...)
	}
	slices.SortStableFunc(out, func(a, b genkind.Append) int { return strings.Compare(a.Path, b.Path) })
	return out
}
