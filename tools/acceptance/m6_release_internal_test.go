package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// m6Calls names, in source order, every method selector runAcceptance
// calls, so the release wiring is read from the code a release runs.
func m6Calls(t *testing.T) []string {
	t.Helper()
	path := filepath.Join(testRoot(t), "tools", "acceptance", "m2_collect.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "runAcceptance" {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			if call, isCall := node.(*ast.CallExpr); isCall {
				if selector, isSelector := call.Fun.(*ast.SelectorExpr); isSelector {
					calls = append(calls, selector.Sel.Name)
				}
			}
			return true
		})
	}
	return calls
}

func TestM6RowsJoinTheReleaseRunAfterTheM5Rows(t *testing.T) {
	t.Parallel()
	calls := m6Calls(t)
	m5, m6 := -1, -1
	for i, name := range calls {
		switch name {
		case "m5AcceptanceRows":
			m5 = i
		case "m6AcceptanceRows":
			m6 = i
		}
	}
	if m5 < 0 || m6 < m5 {
		t.Fatalf("runAcceptance calls %q: the M6 rows must follow the M5 rows", calls)
	}
}
