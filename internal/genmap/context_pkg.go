package genmap

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path"
	"slices"
	"strconv"
	"strings"
)

// srcFile is one parsed Go source the context reads, with the file set its
// lines are counted in, so every declaration answers its file and line.
type srcFile struct {
	path string
	fset *token.FileSet
	file *ast.File
}

// funcDecl is one function or method of a package with its file.
type funcDecl struct {
	src srcFile
	fn  *ast.FuncDecl
}

// typeDecl is one named type of a package with its file.
type typeDecl struct {
	src  srcFile
	spec *ast.TypeSpec
}

// pkgSrc is the parsed sources of one directory, indexed by what the
// context looks up: the types by name, the methods by receiver and the
// functions.
type pkgSrc struct {
	dir     string
	name    string
	srcs    []srcFile
	types   map[string]typeDecl
	methods map[string][]funcDecl
	funcs   []funcDecl
}

// parseSrc parses one file with its positions; false when it does not
// parse, because the context is a guide, never a verdict.
func parseSrc(f goFile) (srcFile, bool) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, f.path, f.content, parser.SkipObjectResolution)
	if err != nil {
		return srcFile{}, false
	}
	return srcFile{path: f.path, fset: fset, file: file}, true
}

// loadPkg parses the non-test sources of one directory.
func loadPkg(files []goFile, dir string) pkgSrc {
	p := pkgSrc{dir: dir, types: map[string]typeDecl{}, methods: map[string][]funcDecl{}}
	for _, f := range files {
		if f.test || path.Dir(f.path) != dir {
			continue
		}
		if s, ok := parseSrc(f); ok {
			addSrc(&p, s)
		}
	}
	return p
}

// addSrc indexes the declarations of one source into the package.
func addSrc(p *pkgSrc, s srcFile) {
	p.srcs, p.name = append(p.srcs, s), s.file.Name.Name
	for _, decl := range s.file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok {
					p.types[ts.Name.Name] = typeDecl{src: s, spec: ts}
				}
			}
		case *ast.FuncDecl:
			if d.Recv != nil && len(d.Recv.List) == 1 {
				recv := recvName(d.Recv.List[0].Type)
				p.methods[recv] = append(p.methods[recv], funcDecl{src: s, fn: d})
				continue
			}
			p.funcs = append(p.funcs, funcDecl{src: s, fn: d})
		}
	}
}

// recvName is the base type of a receiver, its type parameters dropped.
func recvName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	switch e := expr.(type) {
	case *ast.IndexExpr:
		expr = e.X
	case *ast.IndexListExpr:
		expr = e.X
	}
	return typeName(expr)
}

// sortedTypes answers the type names of the package in source order of
// their files and lines, so two runs print the same context.
func (p pkgSrc) sortedTypes() []string {
	names := make([]string, 0, len(p.types))
	for name := range p.types {
		names = append(names, name)
	}
	slices.SortFunc(names, func(a, b string) int { return strings.Compare(p.types[a].at(), p.types[b].at()) })
	return names
}

// ctorOf answers the function of the package that builds the type: a name
// that starts with new or New and a first result of the type or a pointer
// to it.
func (p pkgSrc) ctorOf(typ string) (funcDecl, bool) {
	for _, f := range p.funcs {
		name, res := f.fn.Name.Name, f.fn.Type.Results
		if !strings.HasPrefix(strings.ToLower(name), "new") || res == nil || len(res.List) == 0 {
			continue
		}
		if typeName(res.List[0].Type) == typ {
			return f, true
		}
	}
	return funcDecl{}, false
}

// at is the slash path and line of a node of the file.
func (s srcFile) at(n ast.Node) string {
	return s.path + ":" + strconv.Itoa(s.fset.Position(n.Pos()).Line)
}

// at is where the type is declared.
func (t typeDecl) at() string { return t.src.at(t.spec) }

// text prints an expression on one line.
func (s srcFile) text(e ast.Expr) string {
	var b strings.Builder
	if err := printer.Fprint(&b, s.fset, e); err != nil {
		return ""
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// fieldList writes a parameter or field list as Go declares it, the names
// of one type grouped: "ctx context.Context, id string".
func (s srcFile) fieldList(fl *ast.FieldList, sep string) string {
	if fl == nil {
		return ""
	}
	parts := make([]string, 0, len(fl.List))
	for _, f := range fl.List {
		typ := s.text(f.Type)
		if len(f.Names) == 0 {
			parts = append(parts, typ)
			continue
		}
		names := make([]string, 0, len(f.Names))
		for _, n := range f.Names {
			names = append(names, n.Name)
		}
		parts = append(parts, strings.Join(names, ", ")+" "+typ)
	}
	return strings.Join(parts, sep)
}

// signature writes a function type without its keyword:
// "(ctx context.Context, id string) (Plan, error)".
func (s srcFile) signature(ft *ast.FuncType) string {
	out := "(" + s.fieldList(ft.Params, ", ") + ")"
	res := ft.Results
	if res == nil || len(res.List) == 0 {
		return out
	}
	if len(res.List) == 1 && len(res.List[0].Names) == 0 {
		return out + " " + s.text(res.List[0].Type)
	}
	return out + " (" + s.fieldList(res, ", ") + ")"
}

// funcLine is a function with its signature: "NewPlans(db *DB) *Plans".
func (f funcDecl) funcLine() string {
	return f.fn.Name.Name + f.src.signature(f.fn.Type)
}

// shape writes the type of a declaration on one line: the fields of a
// struct, the methods of an interface, any other type as Go writes it.
func (t typeDecl) shape() string {
	switch typ := t.spec.Type.(type) {
	case *ast.StructType:
		return "struct{" + t.src.fieldList(typ.Fields, "; ") + "}"
	case *ast.InterfaceType:
		return "interface{" + strings.Join(interfaceLines(t.src, typ), "; ") + "}"
	default:
		return t.src.text(t.spec.Type)
	}
}

// interfaceLines writes each method of an interface with its signature
// and each embedded type by name.
func interfaceLines(s srcFile, iface *ast.InterfaceType) []string {
	var out []string
	for _, m := range iface.Methods.List {
		ft, isFunc := m.Type.(*ast.FuncType)
		if !isFunc || len(m.Names) == 0 {
			out = append(out, s.text(m.Type))
			continue
		}
		for _, n := range m.Names {
			out = append(out, n.Name+s.signature(ft))
		}
	}
	return out
}

// importAlias is the name a file gives the import of a path, empty when
// it does not import it.
func importAlias(f *ast.File, importPath string) string {
	for _, spec := range f.Imports {
		got, err := strconv.Unquote(spec.Path.Value)
		if err != nil || got != importPath {
			continue
		}
		if spec.Name != nil {
			return spec.Name.Name
		}
		return path.Base(got)
	}
	return ""
}

// importPathOf is the path a file imports under an alias, empty for none.
func importPathOf(f *ast.File, alias string) string {
	for _, spec := range f.Imports {
		got, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(got)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == alias {
			return got
		}
	}
	return ""
}

// valueSpecs answers the specs of the var or const declarations of a file.
func valueSpecs(f *ast.File, tok token.Token) []*ast.ValueSpec {
	var out []*ast.ValueSpec
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != tok {
			continue
		}
		for _, spec := range gen.Specs {
			if vs, isValue := spec.(*ast.ValueSpec); isValue {
				out = append(out, vs)
			}
		}
	}
	return out
}
