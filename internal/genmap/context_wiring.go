package genmap

import (
	"bytes"
	"go/ast"
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
)

// wireSet is what the composition root may build of the area: per import
// path the constructors worth naming, nil for every New function, and the
// input structs whose literals are worth naming.
type wireSet struct {
	ctors    map[string]map[string]bool
	literals map[string]bool // "<import path>.<Type>"
	local    map[string]bool // the adapter types of the wiring directory
}

// wiringDecls answers where the composition root builds the area's parts:
// each call of a New function of the area or of its handlers, of the
// constructor of each adapter found, and each literal of an input struct
// or of a wiring adapter, at its file and line.
func (r *areaReader) wiringDecls(c Context) []Decl {
	wiringDirs := r.layerDirs(r.in.Policy.Layout.Wiring)
	set := wireSet{ctors: map[string]map[string]bool{}, literals: map[string]bool{}, local: map[string]bool{}}
	for _, dir := range append([]string{r.area.dir}, r.inDirs...) {
		set.ctors[r.dirImport(dir)] = nil
		r.inputLiterals(r.pkg(dir), set.literals)
	}
	for _, a := range c.Adapters {
		dir := path.Dir(atPath(a.At))
		_, typ, _ := strings.Cut(a.Name, ".")
		if slices.Contains(wiringDirs, dir) {
			set.local[typ] = true
			continue
		}
		if ctor, ok := r.pkg(dir).ctorOf(typ); ok {
			imp := r.dirImport(dir)
			if set.ctors[imp] == nil {
				set.ctors[imp] = map[string]bool{}
			}
			set.ctors[imp][ctor.fn.Name.Name] = true
		}
	}
	var out []Decl
	for _, dir := range wiringDirs {
		for _, s := range r.pkg(dir).srcs {
			out = append(out, set.in(s)...)
		}
	}
	return out
}

// inputLiterals adds the structs the New functions of a package take, and
// the structs of the package their fields hold, such as Deps and Settings.
func (r *areaReader) inputLiterals(p pkgSrc, out map[string]bool) {
	imp := r.dirImport(p.dir)
	for _, f := range p.funcs {
		if !strings.HasPrefix(f.fn.Name.Name, wordNew) {
			continue
		}
		for _, param := range f.fn.Type.Params.List {
			t, ok := p.types[typeName(param.Type)]
			if !ok || !isStruct(t) {
				continue
			}
			out[imp+"."+t.spec.Name.Name] = true
			for _, name := range localStructFields(p, t) {
				out[imp+"."+name] = true
			}
		}
	}
}

// localStructFields answers the struct types of the package the fields of
// a struct hold.
func localStructFields(p pkgSrc, t typeDecl) []string {
	st, ok := t.spec.Type.(*ast.StructType)
	if !ok {
		return nil
	}
	var out []string
	for _, field := range st.Fields.List {
		if ft, isLocal := p.types[typeName(field.Type)]; isLocal && isStruct(ft) {
			out = append(out, ft.spec.Name.Name)
		}
	}
	return out
}

// in answers the constructor calls and part literals of one file.
func (w wireSet) in(s srcFile) []Decl {
	paths := map[string]string{}
	for imp := range w.ctors {
		if alias := importAlias(s.file, imp); alias != "" {
			paths[alias] = imp
		}
	}
	var out []Decl
	add := func(name string, n ast.Node) {
		d := Decl{Name: name, At: s.at(n)}
		if !slices.ContainsFunc(out, func(o Decl) bool { return o.Name == d.Name && o.At == d.At }) {
			out = append(out, d)
		}
	}
	ast.Inspect(s.file, func(n ast.Node) bool {
		if name, ok := w.part(n, paths); ok {
			add(name, n)
		}
		return true
	})
	return out
}

// part names a node the wiring section lists: a constructor call, a
// literal of an input struct or of an adapter of the wiring directory.
func (w wireSet) part(n ast.Node, paths map[string]string) (string, bool) {
	switch x := n.(type) {
	case *ast.CallExpr:
		if alias, name, ok := qualified(x.Fun); ok && w.builds(paths[alias], name) {
			return alias + "." + name, true
		}
	case *ast.CompositeLit:
		if len(x.Elts) == 0 {
			return "", false
		}
		if alias, name, ok := qualified(x.Type); ok && w.literals[paths[alias]+"."+name] {
			return alias + "." + name + "{}", true
		}
		if id, isIdent := x.Type.(*ast.Ident); isIdent && w.local[id.Name] {
			return id.Name + "{}", true
		}
	}
	return "", false
}

// builds reports whether a call of the import path's function is one the
// wiring section names.
func (w wireSet) builds(imp, name string) bool {
	names, ok := w.ctors[imp]
	if !ok || imp == "" {
		return false
	}
	if names == nil {
		return strings.HasPrefix(name, wordNew)
	}
	return names[name]
}

// qualified splits a selector of a package: "cockpit.NewUseCases".
func qualified(e ast.Expr) (string, string, bool) {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return "", "", false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", "", false
	}
	return pkg.Name, sel.Sel.Name, true
}

// atPath is the path of a "path:line" position.
func atPath(at string) string {
	if before, _, ok := strings.CutLast(at, ":"); ok {
		return before
	}
	return at
}

// layer is one directory the patterns section reads, with the names a
// test of that layer mentions.
type layer struct {
	label, dir string
	keys       []string
}

// patternDecls answers, for each layer the context found, the test file
// to copy from: the shortest of its directory that mentions a part of the
// layer, else the shortest, among the files with a test and no build tag.
func (r *areaReader) patternDecls(c Context) []Decl {
	wiringDirs := r.layerDirs(r.in.Policy.Layout.Wiring)
	layers := []layer{{label: "usecase " + r.in.Area, dir: r.area.dir, keys: append(slices.Clone(r.cases), "Execute(")}}
	for _, dir := range r.outDirs {
		if !slices.Contains(wiringDirs, dir) {
			layers = append(layers, layer{label: "adapter " + path.Base(dir), dir: dir, keys: typesIn(c.Adapters, dir)})
		}
	}
	for _, dir := range r.inDirs {
		layers = append(layers, layer{label: "handler " + path.Base(dir), dir: dir, keys: []string{"httptest."}})
	}
	for _, w := range c.Wiring {
		dir := path.Dir(atPath(w.At))
		if !slices.ContainsFunc(layers, func(l layer) bool { return l.dir == dir }) {
			layers = append(layers, layer{label: "wiring " + path.Base(dir), dir: dir})
		}
	}
	var out []Decl
	for _, l := range layers {
		if file, ok := r.shortestTest(l); ok {
			out = append(out, Decl{Name: l.label, At: file})
		}
	}
	return out
}

// typesIn answers the type names of the declarations of one directory.
func typesIn(decls []Decl, dir string) []string {
	var out []string
	for _, d := range decls {
		if path.Dir(atPath(d.At)) == dir {
			_, typ, _ := strings.Cut(d.Name, ".")
			out = append(out, typ)
		}
	}
	return out
}

// shortestTest is the smallest test file of the layer's directory that
// mentions one of its keys, else the smallest of all.
func (r *areaReader) shortestTest(l layer) (string, bool) {
	best, size, keyed := "", 0, false
	for _, f := range r.files {
		if !f.test || path.Dir(f.path) != l.dir || !realTest(f.content) {
			continue
		}
		mentions := slices.ContainsFunc(l.keys, func(k string) bool { return bytes.Contains(f.content, []byte(k)) })
		better := best == "" || mentions && !keyed || mentions == keyed && len(f.content) < size
		if better {
			best, size, keyed = f.path, len(f.content), mentions
		}
	}
	return best, best != ""
}

// realTest reports whether a test file declares a test besides TestMain
// and carries no build constraint, so it runs in the plain go test.
func realTest(content []byte) bool {
	tests := bytes.Count(content, []byte("func Test")) - bytes.Count(content, []byte("func TestMain("))
	return tests > 0 && !bytes.Contains(content, []byte("//go:build"))
}

// maxCommands is the count of golaw new lines a generated area receives.
const maxCommands = 5

// commands answers the golaw new commands that fit the area: the port and
// adapter kinds always, the use case and handler kinds when the manifest
// already generates a part of the area.
func (r *areaReader) commands(c Context) []string {
	area := r.in.Area
	sqlPkg := path.Base(path.Dir(r.in.Policy.ThirdParty.SQLCOut))
	if sqlPkg == "." || sqlPkg == "/" {
		sqlPkg = "<pkg>"
	}
	out := make([]string, 0, maxCommands)
	out = append(out,
		"golaw new port "+area+".<Port>: adopts a hand written port or stubs a new one",
		"golaw new adapter sql "+sqlPkg+".<Type> --port "+area+".<Port>: writes the SQL adapter of a port",
		"golaw new adapter http <pkg>.<Type> --port "+area+".<Port>: writes the HTTP client adapter of a port",
	)
	if len(c.Ports) == 0 || !r.generated() {
		return out
	}
	return append(out,
		"golaw new usecase "+area+".<Name> --ports <Port>,<Port> --in <In> --out <Out>: writes a use case with its holes",
		"golaw new handler <pkg> --usecases "+area+": writes the handler of the area's use cases",
	)
}

// generated reports whether the manifest holds a use case of the area.
func (r *areaReader) generated() bool {
	if r.in.Manifest == nil {
		return false
	}
	return slices.ContainsFunc(r.in.Manifest.Entries, func(e manifest.Entry) bool {
		return e.Kind == genkind.KindUsecase && e.Area() == r.in.Area
	})
}
