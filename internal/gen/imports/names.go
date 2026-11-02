package imports

import (
	"go/ast"
	"go/token"
	"path"
	"slices"
	"strconv"
	"strings"
)

// selectorNames answers every identifier written as the X of a selector:
// the only places a package name can be used.
func selectorNames(file *ast.File) map[string]bool {
	used := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				used[id.Name] = true
			}
		}
		return true
	})
	return used
}

// declaredNames answers every name the file declares. A field name is
// counted although it never shadows a package, because a wider local set
// only keeps an import, never removes or adds one wrongly.
func declaredNames(file *ast.File) map[string]bool {
	local := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		for _, id := range declaredIdents(n) {
			if id != nil {
				local[id.Name] = true
			}
		}
		return true
	})
	return local
}

func declaredIdents(n ast.Node) []*ast.Ident {
	switch node := n.(type) {
	case *ast.Field:
		return node.Names
	case *ast.ValueSpec:
		return node.Names
	case *ast.TypeSpec:
		return []*ast.Ident{node.Name}
	case *ast.FuncDecl:
		if node.Recv == nil {
			return []*ast.Ident{node.Name}
		}
	case *ast.AssignStmt:
		if node.Tok == token.DEFINE {
			return identsOf(node.Lhs...)
		}
	case *ast.RangeStmt:
		if node.Tok == token.DEFINE {
			return identsOf(node.Key, node.Value)
		}
	case *ast.LabeledStmt:
		return []*ast.Ident{node.Label}
	}
	return nil
}

func identsOf(exprs ...ast.Expr) []*ast.Ident {
	var out []*ast.Ident
	for _, e := range exprs {
		if id, ok := e.(*ast.Ident); ok {
			out = append(out, id)
		}
	}
	return out
}

// specNames answers the names a spec may be used under, and true for a
// blank or dot import, which the fix never touches. An unnamed import may
// be used under its directory name or its declared package name, which
// differ for some paths, so both are answered.
func specNames(spec *ast.ImportSpec, importPath string, x Index) ([]string, bool) {
	if spec.Name != nil {
		if spec.Name.Name == "_" || spec.Name.Name == "." {
			return nil, true
		}
		return []string{spec.Name.Name}, false
	}
	var names []string
	if last := lastElement(importPath); token.IsIdentifier(last) {
		names = append(names, last)
	}
	if name, ok := x.Name(importPath); ok && !slices.Contains(names, name) {
		names = append(names, name)
	}
	slices.Sort(names)
	return names, false
}

// lastElement skips a major version suffix, the directory a module path
// names its package after.
func lastElement(importPath string) string {
	base := path.Base(importPath)
	if majorVersion(base) && path.Dir(importPath) != "." {
		return path.Base(path.Dir(importPath))
	}
	return base
}

func majorVersion(element string) bool {
	digits, ok := strings.CutPrefix(element, "v")
	if !ok {
		return false
	}
	n, err := strconv.Atoi(digits)
	return err == nil && n >= 2 && strconv.Itoa(n) == digits
}
