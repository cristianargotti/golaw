package port

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"slices"
	"strings"
	"unicode"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
)

// variadic is the prefix go/parser does not read as an expression, so it is
// kept aside and put back after the type is qualified.
const variadic = "..."

// typeExpr is a contract type as the port test package writes it, with the
// package names it uses, so the imports follow from the types alone.
type typeExpr struct {
	Text     string
	Packages []string
}

// qualify rewrites a type of the contract file for a file of another
// package: every name the language does not predeclare and no selector
// carries is the contract package's own, because the contract reader parses
// without types and a bare name can only be declared beside the interface.
func qualify(expr, pkg string) (typeExpr, error) {
	rest, dots := strings.CutPrefix(expr, variadic)
	node, err := parser.ParseExpr(rest)
	if err != nil {
		return typeExpr{}, fmt.Errorf("%w: type %q does not parse: %w", genkind.ErrCannotJudge, expr, err)
	}
	seen := map[string]bool{}
	visit(node, pkg, seen)
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, token.NewFileSet(), node); err != nil {
		return typeExpr{}, fmt.Errorf("%w: type %q does not print: %w", genkind.ErrCannotJudge, expr, err)
	}
	text := buf.String()
	if dots {
		text = variadic + text
	}
	var packages []string
	for name := range seen {
		packages = append(packages, name)
	}
	slices.Sort(packages)
	return typeExpr{Text: text, Packages: packages}, nil
}

// visit renames the contract package's names in place and records every
// package the type names. A field's names are left alone, since a func
// type's parameter names are not types.
func visit(node ast.Node, pkg string, seen map[string]bool) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch e := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := e.X.(*ast.Ident); ok {
				seen[id.Name] = true
			}
			return false
		case *ast.Field:
			visit(e.Type, pkg, seen)
			return false
		case *ast.Ident:
			if types.Universe.Lookup(e.Name) == nil {
				e.Name = pkg + "." + e.Name
				seen[pkg] = true
			}
		}
		return true
	})
}

// lowerFirst lowers the leading initialism of an exported name, so a
// function named after a method reads as Go spells it: HTTPGet is httpGet.
func lowerFirst(name string) string {
	runes := []rune(name)
	run := 0
	for run < len(runes) && unicode.IsUpper(runes[run]) {
		run++
	}
	if run > 1 && run < len(runes) && unicode.IsLower(runes[run]) {
		run--
	}
	for i := range run {
		runes[i] = unicode.ToLower(runes[i])
	}
	return string(runes)
}
