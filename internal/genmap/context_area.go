package genmap

import (
	"cmp"
	"go/ast"
	"go/token"
	"path"
	"slices"
	"strings"
)

// maxRefDepth bounds how far the types section follows a field into the
// types it references: the shapes of the signatures and of their fields.
const maxRefDepth = 2

// typeRef is a type a printed shape references, by directory and name.
type typeRef struct {
	dir, name string
	depth     int
}

// portShape is the method set of a port: each method name with its
// parameter count, which an adapter's methods must match.
type portShape struct {
	name    string
	methods map[string]int
}

// portDecls answers every exported interface of the area with the
// signature of each method.
func (r *areaReader) portDecls() []Decl {
	var out []Decl
	for _, name := range r.area.sortedTypes() {
		t := r.area.types[name]
		iface, ok := t.spec.Type.(*ast.InterfaceType)
		if !ok || !ast.IsExported(name) {
			continue
		}
		r.shown[r.area.dir+"."+name] = true
		r.ports = append(r.ports, shapeOf(name, iface))
		r.refsOf(t.src, iface, 1)
		r.addWords(name)
		out = append(out, Decl{Name: r.in.Area + "." + name, At: t.at(), Lines: interfaceLines(t.src, iface)})
	}
	return out
}

// shapeOf reads the method set of an interface.
func shapeOf(name string, iface *ast.InterfaceType) portShape {
	s := portShape{name: name, methods: map[string]int{}}
	for _, m := range iface.Methods.List {
		ft, isFunc := m.Type.(*ast.FuncType)
		for _, n := range m.Names {
			if isFunc {
				s.methods[n.Name] = arity(ft)
			}
		}
	}
	return s
}

// arity is the parameter count of a function type.
func arity(ft *ast.FuncType) int {
	n := 0
	for _, f := range ft.Params.List {
		n += max(len(f.Names), 1)
	}
	return n
}

// usecaseDecls answers each use case, the exported type with an Execute
// method, then each exported New function of the area that builds no use
// case, such as the constructor of the set of use cases.
func (r *areaReader) usecaseDecls() []Decl {
	for _, name := range r.area.sortedTypes() {
		if ast.IsExported(name) && slices.ContainsFunc(r.area.methods[name], func(f funcDecl) bool { return f.fn.Name.Name == wordExecute }) {
			r.cases = append(r.cases, name)
			r.shown[r.area.dir+"."+name] = true
			r.addWords(name)
		}
	}
	out := make([]Decl, 0, len(r.cases))
	for _, name := range r.cases {
		out = append(out, r.usecaseDecl(name))
	}
	for _, f := range r.area.funcs {
		name := f.fn.Name.Name
		if !strings.HasPrefix(name, wordNew) || f.fn.Recv != nil || r.buildsUsecase(f) {
			continue
		}
		lines := append([]string{f.funcLine()}, r.ioLines(f)...)
		out = append(out, Decl{Name: r.in.Area + "." + name, At: f.src.at(f.fn), Lines: lines})
	}
	return out
}

// buildsUsecase reports whether a function is the constructor of a use case.
func (r *areaReader) buildsUsecase(f funcDecl) bool {
	res := f.fn.Type.Results
	return res != nil && len(res.List) > 0 && slices.Contains(r.cases, typeName(res.List[0].Type))
}

// usecaseDecl writes one use case: its constructor, its exported methods
// with Execute first, and the fields of the structs they take and answer.
func (r *areaReader) usecaseDecl(name string) Decl {
	t := r.area.types[name]
	d := Decl{Name: r.in.Area + "." + name, At: t.at()}
	if ctor, ok := r.area.ctorOf(name); ok {
		d.Lines = append(d.Lines, "new "+ctor.funcLine())
	}
	methods := slices.Clone(r.area.methods[name])
	slices.SortStableFunc(methods, func(a, b funcDecl) int {
		return boolOrder(a.fn.Name.Name != wordExecute, b.fn.Name.Name != wordExecute)
	})
	for _, m := range methods {
		if ast.IsExported(m.fn.Name.Name) {
			d.Lines = append(d.Lines, m.funcLine())
			d.Lines = append(d.Lines, r.ioLines(m)...)
		}
	}
	return d
}

// boolOrder orders false before true.
func boolOrder(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	default:
		return -1
	}
}

// ioLines writes the fields of each struct of the area a function takes
// ("in X struct{...}") or answers ("out Y struct{...}") and collects the
// types its signature references.
func (r *areaReader) ioLines(f funcDecl) []string {
	var out []string
	for _, side := range []struct {
		word string
		list *ast.FieldList
	}{{"in", f.fn.Type.Params}, {"out", f.fn.Type.Results}} {
		if side.list == nil {
			continue
		}
		for _, field := range side.list.List {
			r.refsOf(f.src, field.Type, 1)
			name := typeName(field.Type)
			t, ok := r.area.types[name]
			if !ok || !isStruct(t) || r.shown[r.area.dir+"."+name] {
				continue
			}
			r.shown[r.area.dir+"."+name] = true
			r.refsOf(t.src, t.spec.Type, 1)
			out = append(out, side.word+" "+name+" "+t.shape())
		}
	}
	return out
}

// refsOf collects the named types an expression references: a name of
// the file's own package or a selector of an import inside the module.
func (r *areaReader) refsOf(s srcFile, e ast.Node, depth int) {
	dir := path.Dir(s.path)
	ast.Inspect(e, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if pkg, ok := x.X.(*ast.Ident); ok {
				if d, inModule := r.importDir(importPathOf(s.file, pkg.Name)); inModule {
					r.addRef(typeRef{dir: d, name: x.Sel.Name, depth: depth})
				}
			}
			return false
		case *ast.Ident:
			if ast.IsExported(x.Name) {
				r.addRef(typeRef{dir: dir, name: x.Name, depth: depth})
			}
		}
		return true
	})
}

// addRef keeps a reference once, at its first depth.
func (r *areaReader) addRef(ref typeRef) {
	if !slices.ContainsFunc(r.refs, func(o typeRef) bool { return o.dir == ref.dir && o.name == ref.name }) {
		r.refs = append(r.refs, ref)
	}
}

// typeDecls answers the area's sentinel errors, then every referenced type
// not printed yet with its shape and its named constants, following the
// fields of each up to maxRefDepth.
func (r *areaReader) typeDecls() []Decl {
	var out []Decl
	if d, ok := sentinels(r.in.Area, r.area); ok {
		out = append(out, d)
	}
	for i := 0; i < len(r.refs); i++ { //nolint:intrange // the loop reads the references it appends
		ref := r.refs[i]
		key := ref.dir + "." + ref.name
		p := r.pkg(ref.dir)
		t, ok := p.types[ref.name]
		if r.shown[key] || !ok {
			continue
		}
		r.shown[key] = true
		r.addWords(ref.name)
		d := Decl{Name: p.name + "." + ref.name, At: t.at(), Lines: []string{t.shape()}}
		if c := constsOf(p, ref.name); c != "" {
			d.Lines = append(d.Lines, c)
		}
		if ref.depth < maxRefDepth {
			r.refsOf(t.src, t.spec.Type, ref.depth+1)
		}
		out = append(out, d)
	}
	return out
}

// sentinels answers the Err variables of a package as one declaration.
func sentinels(area string, p pkgSrc) (Decl, bool) {
	d := Decl{Name: area + " errors"}
	var names []string
	for _, s := range p.srcs {
		for _, vs := range valueSpecs(s.file, token.VAR) {
			for _, n := range vs.Names {
				if strings.HasPrefix(n.Name, "Err") {
					names = append(names, n.Name)
					d.At = cmp.Or(d.At, s.at(n))
				}
			}
		}
	}
	d.Lines = []string{strings.Join(names, ", ")}
	return d, len(names) > 0
}
