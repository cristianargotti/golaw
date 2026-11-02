package render

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"slices"
)

// nilExpr is the zero of every type whose values are references.
const nilExpr = "nil"

// Zero is the zero value expression of a type as written, or empty when the
// syntax alone cannot prove it: a named type may be a struct or a number,
// and only the type checker the engine never runs could tell.
func Zero(typ string) string {
	expr, err := parser.ParseExpr(typ)
	if err != nil {
		return ""
	}
	switch e := expr.(type) {
	case *ast.Ident:
		return identZero(e.Name)
	case *ast.StarExpr, *ast.MapType, *ast.ChanType, *ast.FuncType, *ast.InterfaceType:
		return nilExpr
	case *ast.ArrayType:
		if e.Len == nil {
			return nilExpr
		}
		return typ + "{}"
	case *ast.StructType:
		return typ + "{}"
	}
	return ""
}

// identZero answers the zero of a predeclared type name.
func identZero(name string) string {
	switch name {
	case "bool":
		return "false"
	case "string":
		return `""`
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float32", "float64", "complex64", "complex128", "byte", "rune":
		return "0"
	case errorType, "any":
		return nilExpr
	}
	return ""
}

// Qualify prefixes every exported type name of typ with pkg, so a contract
// type written inside its own package reads right from another one.
func Qualify(pkg, typ string) (string, error) {
	if pkg == "" {
		return typ, nil
	}
	expr, err := parser.ParseExpr(typ)
	if err != nil {
		return "", fmt.Errorf("%w: qualify %q: %w", ErrTemplate, typ, err)
	}
	var parents []ast.Node
	ast.Inspect(expr, func(n ast.Node) bool {
		if n == nil {
			parents = parents[:len(parents)-1]
			return false
		}
		if id, ok := n.(*ast.Ident); ok && id.IsExported() && qualifiable(id, parents) {
			id.Name = pkg + "." + id.Name
		}
		parents = append(parents, n)
		return true
	})
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), expr); err != nil {
		return "", fmt.Errorf("%w: qualify %q: %w", ErrTemplate, typ, err)
	}
	return buf.String(), nil
}

// qualifiable reports an identifier that names a type: not a part of a
// selector, which is qualified already, and not a field or method name.
func qualifiable(id *ast.Ident, parents []ast.Node) bool {
	if len(parents) == 0 {
		return true
	}
	switch p := parents[len(parents)-1].(type) {
	case *ast.SelectorExpr:
		return false
	case *ast.Field:
		return !slices.Contains(p.Names, id)
	}
	return true
}
