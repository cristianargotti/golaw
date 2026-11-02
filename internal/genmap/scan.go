package genmap

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/contract"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/model"
)

// goFile is one Go source of the snapshot the map reads.
type goFile struct {
	path    string
	content []byte
	test    bool
}

// goFiles answers the Go sources a person or an agent writes: tracked,
// untracked and test files whose bytes read. Third party output is
// ClassGenerated and left out, because its ports and types are the
// generator's, never the repository's map.
func goFiles(s model.Snapshot) []goFile {
	var out []goFile
	for _, f := range s.Files() {
		if !strings.HasSuffix(f.Path, ".go") {
			continue
		}
		if f.Class != model.ClassTracked && f.Class != model.ClassUntracked && f.Class != model.ClassTest {
			continue
		}
		content, err := s.Read(f.Path)
		if err != nil {
			continue
		}
		out = append(out, goFile{path: f.Path, content: content, test: f.Class == model.ClassTest})
	}
	return out
}

// openHoles answers the holes of every file that may hold one; a file that
// does not parse is skipped, because the map is a guide, never a verdict.
func openHoles(files []goFile) []holes.Hole {
	var out []holes.Hole
	for _, f := range files {
		if !bytes.Contains(f.content, []byte(holes.Marker)) && !bytes.Contains(f.content, []byte("Hole(")) {
			continue
		}
		found, err := holes.Find(f.path, f.content)
		if err != nil {
			continue
		}
		out = append(out, found...)
	}
	return out
}

// ports answers the interfaces of the files of a port directory, each as
// "<area>.<Name>"; the interfaces only types could judge are left out by
// contract.Ports, which still answers the ones beside them.
func ports(files []goFile, pattern string) []Item {
	var out []Item
	for _, f := range files {
		area, ok := matchLayout(pattern, path.Dir(f.path))
		if f.test || !ok {
			continue
		}
		found, err := contract.Ports(f.path, f.content)
		if err != nil && !errors.Is(err, genkind.ErrCannotJudge) {
			continue
		}
		for i := range found {
			out = append(out, portItem(area, f.path, found[i].Name, found[i].Methods))
		}
	}
	return out
}

// parsedUsecases answers a use case for each method named Execute of a
// file of a use case directory, named by its receiver.
func parsedUsecases(files []goFile, pattern string) []Item {
	var out []Item
	for _, f := range files {
		area, ok := matchLayout(pattern, path.Dir(f.path))
		if f.test || !ok {
			continue
		}
		for _, recv := range executeReceivers(parse(f)) {
			out = append(out, Item{Name: area + "." + recv, Path: f.path})
		}
	}
	return out
}

// executeReceivers answers the receiver types of the Execute methods of a
// file, the shape every use case of the layout has.
func executeReceivers(file *ast.File) []string {
	if file == nil {
		return nil
	}
	var out []string
	for _, decl := range file.Decls {
		fn, isFunc := decl.(*ast.FuncDecl)
		if isFunc && fn.Name.Name == "Execute" && fn.Recv != nil && len(fn.Recv.List) == 1 {
			if recv := typeName(fn.Recv.List[0].Type); recv != "" {
				out = append(out, recv)
			}
		}
	}
	return out
}

// assertion is one compile time assertion "var _ X.Y = (*T)(nil)".
type assertion struct {
	iface, impl, pkg string
}

// assertions answers every compile time assertion of a file that names an
// interface of another package, through go/parser only.
func assertions(f goFile) []assertion {
	file := parse(f)
	if file == nil {
		return nil
	}
	var out []assertion
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			if a, ok := assertionOf(spec); ok {
				a.pkg = file.Name.Name
				out = append(out, a)
			}
		}
	}
	return out
}

// assertionOf reads one var spec as an assertion of a selector interface
// by a pointer conversion or the address of a composite literal.
func assertionOf(spec ast.Spec) (assertion, bool) {
	vs, ok := spec.(*ast.ValueSpec)
	if !ok || len(vs.Names) != 1 || vs.Names[0].Name != "_" || len(vs.Values) != 1 {
		return assertion{}, false
	}
	sel, ok := vs.Type.(*ast.SelectorExpr)
	if !ok {
		return assertion{}, false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return assertion{}, false
	}
	impl := ""
	switch v := vs.Values[0].(type) {
	case *ast.CallExpr:
		if paren, isParen := v.Fun.(*ast.ParenExpr); isParen {
			impl = typeName(paren.X)
		}
	case *ast.UnaryExpr:
		if lit, isLit := v.X.(*ast.CompositeLit); isLit && v.Op == token.AND {
			impl = typeName(lit.Type)
		}
	}
	return assertion{iface: pkg.Name + "." + sel.Sel.Name, impl: impl}, impl != ""
}

// typeName is the name of a type or of a pointer to it.
func typeName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// parse reads one file without its comments; nil when it does not parse.
func parse(f goFile) *ast.File {
	file, err := parser.ParseFile(token.NewFileSet(), f.path, f.content, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	return file
}
