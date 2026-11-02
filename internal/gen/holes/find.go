package holes

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
)

// Find answers the holes of one owned file in line order: every certain
// marker and every certain hole call, one entry per id.
func Find(path string, content []byte) ([]Hole, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, content, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("find holes in %s: %w", path, err)
	}
	seen := map[string]bool{}
	var out []Hole
	add := func(id, constraint string, pos token.Pos) {
		if seen[id] {
			return
		}
		seen[id] = true
		out = append(out, Hole{
			ID: id, Constraint: constraint, Path: path,
			Line: fset.Position(pos).Line, Func: enclosing(file, pos),
		})
	}
	for _, group := range file.Comments {
		for _, c := range group.List {
			if id, constraint, ok := marker(c.Text); ok {
				add(id, constraint, c.Pos())
			}
		}
	}
	names := supportNames(file)
	ast.Inspect(file, func(n ast.Node) bool {
		if id, ok := holeCall(n, names); ok {
			add(id, "", n.Pos())
		}
		return true
	})
	slices.SortStableFunc(out, func(a, b Hole) int { return cmp.Compare(a.Line, b.Line) })
	return out, nil
}

// marker reads one comment as a hole marker: a line comment whose text,
// after one optional space, is the marker, a space and a valid id.
func marker(text string) (string, string, bool) {
	body, ok := strings.CutPrefix(text, "//")
	if !ok {
		return "", "", false
	}
	body = strings.TrimPrefix(body, " ")
	rest, ok := strings.CutPrefix(body, Marker+" ")
	if !ok {
		return "", "", false
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 || !validID(fields[0]) {
		return "", "", false
	}
	_, constraint, _ := strings.Cut(strings.TrimSpace(rest), fields[0])
	return fields[0], strings.TrimSpace(constraint), true
}

// holeCall reads a node as a call of the support package's Hole, or of
// HoleOf with its type argument, with a literal valid id as its first
// argument; any other shape is not certain.
func holeCall(n ast.Node, names map[string]bool) (string, bool) {
	call, ok := n.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return "", false
	}
	fun, name := call.Fun, "Hole"
	if index, generic := fun.(*ast.IndexExpr); generic {
		fun, name = index.X, "HoleOf"
	}
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return "", false
	}
	if x, isIdent := sel.X.(*ast.Ident); !isIdent || !names[x.Name] {
		return "", false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	id, err := strconv.Unquote(lit.Value)
	if err != nil || !validID(id) {
		return "", false
	}
	return id, true
}

// supportNames answers the local names of the imports that resolve to the
// support package name.
func supportNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		local := importPath[strings.LastIndex(importPath, "/")+1:]
		if spec.Name != nil {
			local = spec.Name.Name
		}
		if local == SupportName {
			names[local] = true
		}
	}
	return names
}

// enclosing names the top level declaration whose range holds pos, so a
// hole line points the agent at the function or type to fill.
func enclosing(file *ast.File, pos token.Pos) string {
	for _, decl := range file.Decls {
		if pos >= decl.Pos() && pos < decl.End() {
			return declName(decl)
		}
	}
	return ""
}

// declName is a function's name, a method's "(*T).Name" or the first type
// name of a type declaration; any other declaration has no name to give.
func declName(decl ast.Decl) string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Recv == nil || len(d.Recv.List) == 0 {
			return d.Name.Name
		}
		return "(" + receiver(d.Recv.List[0].Type) + ")." + d.Name.Name
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok {
				return ts.Name.Name
			}
		}
	}
	return ""
}

// receiver spells a receiver type as written, its type arguments dropped.
func receiver(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return "*" + receiver(e.X)
	case *ast.ParenExpr:
		return receiver(e.X)
	case *ast.IndexExpr:
		return receiver(e.X)
	case *ast.IndexListExpr:
		return receiver(e.X)
	case *ast.Ident:
		return e.Name
	}
	return ""
}
