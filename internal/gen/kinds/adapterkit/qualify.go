package adapterkit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
)

// qualifier walks one type expression without types: it records where the
// contract's own exported names start, so the adapter's package can name
// them, and the package names of the selectors, so the imports follow.
type qualifier struct {
	expr    string
	pkg     string
	fset    *token.FileSet
	offsets []int
	pkgs    []string
}

// qualify answers expr with pkg prefixed to each exported name of the
// contract's package and the packages the expression selects from. A shape
// the syntax alone cannot place, an inline struct or a generic
// instantiation, is refused instead of guessed (M7.3.1).
func qualify(expr, pkg string) (string, []string, error) {
	if rest, variadic := strings.CutPrefix(expr, "..."); variadic {
		// A variadic parameter is no expression on its own, so its element
		// type is qualified and the ellipsis written back in front of it.
		out, pkgs, err := qualify(rest, pkg)
		return "..." + out, pkgs, err
	}
	q := &qualifier{expr: expr, pkg: pkg, fset: token.NewFileSet()}
	node, err := parser.ParseExprFrom(q.fset, "", expr, 0)
	if err != nil {
		return "", nil, fmt.Errorf("%w: type %s does not parse: %w", genkind.ErrCannotJudge, expr, err)
	}
	if err := q.walk(node); err != nil {
		return "", nil, err
	}
	out := expr
	for _, at := range slices.Backward(q.offsets) {
		out = out[:at] + pkg + "." + out[at:]
	}
	return out, q.pkgs, nil
}

func (q *qualifier) walk(node ast.Expr) error {
	switch n := node.(type) {
	case *ast.Ident:
		return q.ident(n)
	case *ast.SelectorExpr:
		return q.selector(n)
	case *ast.StarExpr:
		return q.walk(n.X)
	case *ast.ArrayType:
		return q.array(n)
	case *ast.MapType:
		return q.all(n.Key, n.Value)
	case *ast.ChanType:
		return q.walk(n.Value)
	case *ast.Ellipsis:
		return q.walk(n.Elt)
	case *ast.FuncType:
		return q.function(n)
	}
	return q.inline(node)
}

func (q *qualifier) all(exprs ...ast.Expr) error {
	for _, e := range exprs {
		if err := q.walk(e); err != nil {
			return err
		}
	}
	return nil
}

// ident keeps a predeclared name and records an exported one; an unexported
// name of the contract's package is unreachable from the adapter's package.
func (q *qualifier) ident(id *ast.Ident) error {
	if types.Universe.Lookup(id.Name) != nil {
		return nil
	}
	if !id.IsExported() {
		return fmt.Errorf("%w: type %s of %s is not exported by package %s", genkind.ErrCannotJudge, id.Name, q.expr, q.pkg)
	}
	q.offsets = append(q.offsets, q.fset.Position(id.Pos()).Offset)
	return nil
}

// selector records the package a qualified name comes from and does not
// descend, because the name after the dot is that package's own.
func (q *qualifier) selector(sel *ast.SelectorExpr) error {
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return q.inline(sel)
	}
	q.pkgs = append(q.pkgs, pkg.Name)
	return nil
}

// array walks the length too, since a named constant length is a name of
// the contract's package like any type.
func (q *qualifier) array(a *ast.ArrayType) error {
	if a.Len != nil {
		if _, literal := a.Len.(*ast.BasicLit); !literal {
			if err := q.walk(a.Len); err != nil {
				return err
			}
		}
	}
	return q.walk(a.Elt)
}

func (q *qualifier) function(fn *ast.FuncType) error {
	for _, list := range []*ast.FieldList{fn.Params, fn.Results} {
		if list == nil {
			continue
		}
		for _, field := range list.List {
			if err := q.walk(field.Type); err != nil {
				return err
			}
		}
	}
	return nil
}

func (q *qualifier) inline(node ast.Expr) error {
	at := q.fset.Position(node.Pos()).Offset
	end := q.fset.Position(node.End()).Offset
	return fmt.Errorf("%w: %s is written inline in %s", genkind.ErrCannotJudge, q.expr[at:end], q.expr)
}
