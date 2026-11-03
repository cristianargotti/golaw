package genmap

import (
	"go/ast"
	"path"
	"slices"
	"strings"
)

// handlerDecls answers the inbound handlers of the area: in every handler
// directory whose package imports the area, each type that serves a route
// or calls a use case, with its constructor, the structs the constructor
// takes and its routes.
func (r *areaReader) handlerDecls() []Decl {
	fields := r.usecaseFields()
	var out []Decl
	for _, dir := range r.layerDirs(r.in.Policy.Layout.Handler) {
		p := r.pkg(dir)
		if !r.importsArea(p) {
			continue
		}
		decls := handlerPkgDecls(p, fields)
		if len(decls) > 0 {
			r.inDirs = append(r.inDirs, dir)
			out = append(out, decls...)
		}
	}
	return out
}

// usecaseFields maps the field of an area struct that holds a use case to
// the use case: "Plans" to "ListPlans".
func (r *areaReader) usecaseFields() map[string]string {
	out := map[string]string{}
	for _, name := range r.area.sortedTypes() {
		st, ok := r.area.types[name].spec.Type.(*ast.StructType)
		if !ok {
			continue
		}
		for _, f := range st.Fields.List {
			uc := typeName(f.Type)
			for _, n := range f.Names {
				if _, taken := out[n.Name]; !taken && slices.Contains(r.cases, uc) {
					out[n.Name] = uc
				}
			}
		}
	}
	return out
}

// handlerPkgDecls writes the handler types of one package.
func handlerPkgDecls(p pkgSrc, fields map[string]string) []Decl {
	routes := routesOf(p)
	var types []string
	for _, typ := range p.sortedTypes() {
		serves := slices.ContainsFunc(p.methods[typ], func(m funcDecl) bool {
			return len(usecaseCalls(m.fn, fields)) > 0 || slices.ContainsFunc(routes, func(rt route) bool { return rt.target == m.fn.Name.Name })
		})
		if serves {
			types = append(types, typ)
		}
	}
	out := make([]Decl, 0, len(types))
	for i, typ := range types {
		out = append(out, handlerDecl(p, typ, routes, fields, i == 0))
	}
	return out
}

// handlerDecl writes one handler type: its constructor and the structs it
// takes, each route it serves with the use cases the route calls, then
// each other method that calls a use case. The first type of the package
// also carries the routes of the package's free functions.
func handlerDecl(p pkgSrc, typ string, routes []route, fields map[string]string, first bool) Decl {
	d := Decl{Name: p.name + "." + typ, At: p.types[typ].at()}
	if ctor, ok := p.ctorOf(typ); ok {
		d.Lines = append(d.Lines, "new "+ctor.funcLine())
		d.Lines = append(d.Lines, localStructs(p, ctor.fn.Type.Params)...)
	}
	methods := map[string]funcDecl{}
	for _, m := range p.methods[typ] {
		methods[m.fn.Name.Name] = m
	}
	routed := map[string]bool{}
	for _, rt := range routes {
		m, isMethod := methods[rt.target]
		if !isMethod && (!first || !isFunc(p, rt.target)) {
			continue
		}
		line := rt.line()
		if isMethod {
			routed[rt.target] = true
			line += " " + path.Base(m.src.at(m.fn)) + calls(usecaseCalls(m.fn, fields))
		}
		d.Lines = append(d.Lines, line)
	}
	for _, m := range p.methods[typ] {
		if uses := usecaseCalls(m.fn, fields); !routed[m.fn.Name.Name] && len(uses) > 0 {
			d.Lines = append(d.Lines, m.funcLine()+calls(uses))
		}
	}
	return d
}

// calls writes the use cases a handler calls: " uses ListPlans".
func calls(uses []string) string {
	if len(uses) == 0 {
		return ""
	}
	return " uses " + strings.Join(uses, ", ")
}

// isFunc reports whether the package declares a free function of the name.
func isFunc(p pkgSrc, name string) bool {
	return slices.ContainsFunc(p.funcs, func(f funcDecl) bool { return f.fn.Name.Name == name })
}

// localStructs writes the fields of each struct of the package a
// parameter list takes: "in Deps struct{...}".
func localStructs(p pkgSrc, params *ast.FieldList) []string {
	var out []string
	for _, f := range params.List {
		name := typeName(f.Type)
		if t, ok := p.types[name]; ok && isStruct(t) {
			out = append(out, "in "+name+" "+t.shape())
		}
	}
	return out
}

// isStruct reports whether a declared type is a struct.
func isStruct(t typeDecl) bool {
	_, ok := t.spec.Type.(*ast.StructType)
	return ok
}
