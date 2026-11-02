package adaptersql

import (
	"slices"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adapterkit"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/model"
)

// again renders an existing instance: the regenerable files in full, and for
// owned code only appends, because owned code is the agent's (M7.4.2).
func (p *sqlPlan) again(in genkind.Input, entry manifest.Entry) (genkind.Output, error) {
	r, err := newRenderer(p, in)
	if err != nil {
		return genkind.Output{}, err
	}
	regenerable, err := r.regenerable()
	if err != nil {
		return genkind.Output{}, err
	}
	shaped := make([]adapterkit.Method, 0, len(p.data.Methods))
	for i := range p.data.Methods {
		shaped = append(shaped, p.data.Methods[i].Method)
	}
	missing, decisions := adapterkit.Reconcile(p.target, shaped, p.pkg, entry.Owned)
	next := entry
	next.Contracts, next.Regenerable = p.contracts, paths(regenerable)
	out := genkind.Output{Regenerable: regenerable, Decisions: decisions}
	if err := r.appends(&out, &next, p.missing(missing)); err != nil {
		return genkind.Output{}, err
	}
	out.Entry = next
	out.Bindings = p.bindings(in, next)
	return out, nil
}

// missing answers the planned methods of the names Reconcile found missing.
func (p *sqlPlan) missing(shaped []adapterkit.Method) []method {
	var out []method
	for i := range p.data.Methods {
		name := p.data.Methods[i].Name
		if slices.ContainsFunc(shaped, func(s adapterkit.Method) bool { return s.Name == name }) {
			out = append(out, p.data.Methods[i])
		}
	}
	return out
}

// appends adds to out the stubs and red tests owned code lacks; a gone main
// file is named instead, since golaw never writes owned code over a deletion.
func (r renderer) appends(out *genkind.Output, entry *manifest.Entry, methods []method) error {
	fresh := r.p.newMappers
	if len(methods) == 0 && len(fresh) == 0 {
		return nil
	}
	main := r.file(r.p.data.Lower + ".go")
	if !r.present(main) {
		out.Decisions = append(out.Decisions, genkind.Decision{
			Path: main, Line: 1, Text: main + " is gone: restore it, golaw never rewrites owned code",
		})
		return nil
	}
	decls, ids, err := r.stubs(methods, fresh)
	if err != nil {
		return err
	}
	out.Appends = append(out.Appends, genkind.Append{Path: main, Decls: decls, Imports: r.mainImports(methods, fresh), Holes: ids})
	if len(methods) > 0 {
		reds, redErr := r.reds(methods)
		if redErr != nil {
			return redErr
		}
		out.Appends = append(out.Appends, genkind.Append{Path: r.file(r.p.data.Lower + "_test.go"), Decls: reds, Imports: r.testImports(methods)})
	}
	if len(fresh) > 0 {
		return r.mapperAppend(out, entry)
	}
	return nil
}

// mapperAppend appends the red tests of fresh mappers to the internal test
// file, or creates that file when the instance never had one.
func (r renderer) mapperAppend(out *genkind.Output, entry *manifest.Entry) error {
	internal := r.file(r.p.data.Lower + "_internal_test.go")
	if r.present(internal) {
		reds, err := r.mapperReds(r.p.newMappers)
		if err != nil {
			return err
		}
		out.Appends = append(out.Appends, genkind.Append{Path: internal, Decls: reds, Imports: r.internalImports()})
		return nil
	}
	f, err := r.internalFile()
	if err != nil {
		return err
	}
	out.Owned = append(out.Owned, f)
	if !slices.Contains(entry.Owned, internal) {
		entry.Owned = append(slices.Clone(entry.Owned), internal)
		slices.Sort(entry.Owned)
	}
	return nil
}

// present reports whether the snapshot holds rel.
func (r renderer) present(rel string) bool {
	return slices.ContainsFunc(r.in.Snapshot.Files(), func(f model.File) bool { return f.Path == rel })
}
