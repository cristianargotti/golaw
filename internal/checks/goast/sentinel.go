package goast

import (
	"go/ast"
	"go/token"
	"path"

	"github.com/cristianargotti/golaw/internal/model"
)

const codeSentinelDecl = "sentinel-decl"

// SentinelDecl is goast.sentinel-decl: a package declares its failure
// modes once, in errors.go, so a package-level errors.New anywhere else is
// a finding.
func SentinelDecl() model.Check {
	return check{spec: spec("sentinel-decl"), judge: judgeSentinelDecl}
}

func judgeSentinelDecl(t *target, _ *state) ([]model.Finding, error) {
	if path.Base(t.file.Path) == errorsFile {
		return nil, nil
	}
	file, _, err := t.parsed()
	if err != nil {
		return nil, err
	}
	var out []model.Finding
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, value := range sentinelValues(gen) {
			message := "a sentinel is declared outside errors.go: a package declares its failure modes once"
			out = append(out, t.finding(codeSentinelDecl, value.Pos(), message))
		}
	}
	return out, nil
}

// sentinelValues lists the errors.New calls of a var declaration.
func sentinelValues(gen *ast.GenDecl) []ast.Expr {
	var out []ast.Expr
	for _, s := range gen.Specs {
		spec, ok := s.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, value := range spec.Values {
			if call, ok := value.(*ast.CallExpr); ok && callee(call) == "errors.New" {
				out = append(out, value)
			}
		}
	}
	return out
}
