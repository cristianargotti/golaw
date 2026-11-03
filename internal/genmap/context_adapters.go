package genmap

import (
	"go/ast"
	"go/token"
	"path"
	"slices"
	"strings"
)

// adapterDecls answers the outbound adapters of the area: every type of an
// adapter or wiring directory that asserts one of its ports, has the
// methods of one, or, in an SQL adapter directory, calls one of the area's
// queries, with its constructor and the reason it belongs.
func (r *areaReader) adapterDecls(queries []queryFile) []Decl {
	l := r.in.Policy.Layout
	names := map[string]string{}
	for _, qf := range queries {
		for _, q := range qf.queries {
			names[q.name] = path.Base(qf.path)
		}
	}
	var out []Decl
	for _, dir := range r.layerDirs(l.AdapterSQL, l.AdapterHTTP, l.Wiring) {
		p := r.pkg(dir)
		called := names
		if _, sql := matchLayout(l.AdapterSQL, dir); !sql {
			called = nil
		}
		asserted := r.assertedPorts(p)
		found := false
		for _, typ := range p.sortedTypes() {
			why := append(slices.Clone(asserted[typ]), r.fittedPorts(p, typ, asserted[typ])...)
			why = append(why, calledQueries(p.methods[typ], called)...)
			if len(why) == 0 {
				continue
			}
			found = true
			out = append(out, adapterDecl(p, typ, why))
		}
		if found {
			r.outDirs = append(r.outDirs, dir)
		}
	}
	return out
}

// adapterDecl writes one adapter with its constructor and its reasons.
func adapterDecl(p pkgSrc, typ string, why []string) Decl {
	d := Decl{Name: p.name + "." + typ, At: p.types[typ].at()}
	if ctor, ok := p.ctorOf(typ); ok {
		d.Lines = append(d.Lines, "new "+ctor.funcLine())
	}
	d.Lines = append(d.Lines, why...)
	return d
}

// layerDirs answers the directories of the non-test Go sources that one of
// the layout patterns places, sorted.
func (r *areaReader) layerDirs(patterns ...string) []string {
	var out []string
	for _, f := range r.files {
		dir := path.Dir(f.path)
		if f.test || slices.Contains(out, dir) {
			continue
		}
		for _, pattern := range patterns {
			if _, ok := matchLayout(pattern, dir); ok {
				out = append(out, dir)
				break
			}
		}
	}
	slices.Sort(out)
	return out
}

// assertedPorts answers, by type, the area's ports a compile time
// assertion of the package names.
func (r *areaReader) assertedPorts(p pkgSrc) map[string][]string {
	out := map[string][]string{}
	for _, s := range p.srcs {
		alias := importAlias(s.file, r.areaImport())
		if alias == "" {
			continue
		}
		for _, vs := range valueSpecs(s.file, token.VAR) {
			a, isAssertion := assertionOf(vs)
			port, ofArea := strings.CutPrefix(a.iface, alias+".")
			if isAssertion && ofArea && r.isPort(port) {
				out[a.impl] = append(out[a.impl], wordPort+" "+r.in.Area+"."+port+" (asserted)")
			}
		}
	}
	return out
}

// isPort reports whether a name is a port of the area.
func (r *areaReader) isPort(name string) bool {
	return slices.ContainsFunc(r.ports, func(s portShape) bool { return s.name == name })
}

// fittedPorts answers the ports whose every method the type has with the
// same parameter count, the ones an assertion named left out.
func (r *areaReader) fittedPorts(p pkgSrc, typ string, asserted []string) []string {
	have := map[string]int{}
	for _, m := range p.methods[typ] {
		have[m.fn.Name.Name] = arity(m.fn.Type)
	}
	var out []string
	for _, port := range r.ports {
		if weak(port) || slices.ContainsFunc(asserted, func(a string) bool { return strings.Contains(a, "."+port.name+" ") }) {
			continue
		}
		fits := true
		for name, n := range port.methods {
			if got, ok := have[name]; !ok || got != n {
				fits = false
				break
			}
		}
		if fits {
			out = append(out, wordPort+" "+r.in.Area+"."+port.name+" (methods)")
		}
	}
	return out
}

// calledQueries answers the area's query files whose queries the methods
// call, as one "queries a.sql, b.sql" line.
func calledQueries(methods []funcDecl, names map[string]string) []string {
	if len(names) == 0 {
		return nil
	}
	var files []string
	for _, m := range methods {
		ast.Inspect(m.fn, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, isSel := call.Fun.(*ast.SelectorExpr); isSel && names[sel.Sel.Name] != "" && !slices.Contains(files, names[sel.Sel.Name]) {
				files = append(files, names[sel.Sel.Name])
			}
			return true
		})
	}
	if len(files) == 0 {
		return nil
	}
	slices.Sort(files)
	return []string{"queries " + strings.Join(files, ", ")}
}

// importsArea reports whether a source of the package imports the area.
func (r *areaReader) importsArea(p pkgSrc) bool {
	return slices.ContainsFunc(p.srcs, func(s srcFile) bool { return importAlias(s.file, r.areaImport()) != "" })
}

// weak reports whether a port's method set is too plain to judge a type
// by its method names: no method, or only methods without parameters,
// such as Now of a clock, which many unrelated types have.
func weak(port portShape) bool {
	for _, n := range port.methods {
		if n > 0 {
			return false
		}
	}
	return true
}
