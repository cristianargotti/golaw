package goast

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	codeIdenticalFunctions = "identical-functions"
	identicalMinStatements = 2
)

// SonarIdenticalFunctions is goast.sonar-identical-functions (Sonar rule
// S4144): two functions of one file with the same receiver, the same
// type parameters, the same parameters and the same body of two or more
// statements are a finding on the later one.
func SonarIdenticalFunctions() model.Check {
	return check{spec: spec("sonar-identical-functions"), judge: judgeIdenticalFunctions}
}

func judgeIdenticalFunctions(t *target, _ *state) ([]model.Finding, error) {
	file, fset, err := t.parsed()
	if err != nil {
		return nil, err
	}
	seen := map[string]*ast.FuncDecl{}
	var out []model.Finding
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || len(fn.Body.List) < identicalMinStatements {
			continue
		}
		shape, err := functionShape(fset, fn)
		if err != nil {
			return nil, err
		}
		other, dup := seen[shape]
		if !dup {
			seen[shape] = fn
			continue
		}
		message := fmt.Sprintf("function %s has the receiver, the parameters and the body of %s: keep one", fn.Name.Name, other.Name.Name)
		out = append(out, t.finding(codeIdenticalFunctions, fn.Name.Pos(), message))
	}
	return out, nil
}

// functionShape prints the receiver type, the type parameters, the
// parameters and the body of a function without its comments, so two
// functions that differ only in name, result names or formatting print the
// same.
func functionShape(fset *token.FileSet, fn *ast.FuncDecl) (string, error) {
	signature := &ast.FuncType{TypeParams: fn.Type.TypeParams, Params: fn.Type.Params}
	parts := []ast.Node{signature, fn.Body}
	if fn.Recv != nil {
		parts = append(parts, fn.Recv.List[0].Type)
	}
	printed := make([]string, 0, len(parts))
	for _, part := range parts {
		var buf bytes.Buffer
		if err := printer.Fprint(&buf, fset, part); err != nil {
			return "", fmt.Errorf("print %s: %w", fn.Name.Name, err)
		}
		printed = append(printed, buf.String())
	}
	return strings.Join(printed, "\n"), nil
}
