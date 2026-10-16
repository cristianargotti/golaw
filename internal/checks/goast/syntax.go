package goast

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

// selector answers only for X.Sel on a plain identifier, the shape of a
// package-qualified name the package rules of the family look for.
func selector(n ast.Node) (string, string, bool) {
	sel, ok := n.(*ast.SelectorExpr)
	if !ok {
		return "", "", false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", "", false
	}
	return ident.Name, sel.Sel.Name, true
}

// callee spells a call the way the rules list names, since the check
// resolves no type: pkg.Name for a selector on an identifier, Name for a
// plain identifier, .Name for a method on anything else, and empty for a
// call of an expression.
func callee(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		if ident, ok := fun.X.(*ast.Ident); ok {
			return ident.Name + "." + fun.Sel.Name
		}
		return "." + fun.Sel.Name
	default:
		return ""
	}
}

// lastSegment drops the package or the receiver, so a rule matches a call
// by its name alone.
func lastSegment(name string) string {
	if _, after, ok := strings.CutLast(name, "."); ok {
		return after
	}
	return name
}

// typeName drops the package and the pointer, so a method on *T and a
// literal of pkg.T both name T; any other shape names nothing.
func typeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.StarExpr:
		return typeName(e.X)
	default:
		return ""
	}
}

// commentLines lists the lines that hold a comment opening with the marker
// and going on with text. A marker stands for the reason a reader needs,
// so a bare marker, or one inside a sentence, clears nothing.
func commentLines(file *ast.File, fset *token.FileSet, marker string) map[int]bool {
	lines := map[int]bool{}
	for _, group := range file.Comments {
		for _, c := range group.List {
			if justifies(c.Text, marker) {
				lines[fset.Position(c.Pos()).Line] = true
			}
		}
	}
	return lines
}

// justifies strips the comment delimiters first, so a block comment
// carries a marker the way a line comment does.
func justifies(comment, marker string) bool {
	text := strings.TrimPrefix(comment, "//")
	if body, ok := strings.CutPrefix(comment, "/*"); ok {
		text = strings.TrimSuffix(body, "*/")
	}
	rest, ok := strings.CutPrefix(strings.TrimSpace(text), marker)
	return ok && strings.TrimSpace(rest) != ""
}

// stringLiteral unquotes the literal, so a rule matches its value and not
// its spelling with quotes and escapes.
func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}
