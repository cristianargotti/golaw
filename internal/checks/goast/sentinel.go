package goast

import (
	"go/ast"
	"go/token"
	"path"
	"strings"

	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/model"
)

const codeSentinelDecl = "sentinel-decl"

// The support package a golaw kind writes keeps its sentinels in its
// generated errors files, the only Go source the kit writes into a
// repository (M7.4.3), so those files hold the role errors.go holds.
const golawErrorsName = "zz_generated_errors"

// SentinelDecl is goast.sentinel-decl: a package declares its failure
// modes once, in errors.go, so a package-level errors.New anywhere else is
// a finding.
func SentinelDecl() model.Check {
	return check{spec: spec("sentinel-decl"), judge: judgeSentinelDecl}
}

func judgeSentinelDecl(t *target, _ *state) ([]model.Finding, error) {
	if path.Base(t.file.Path) == errorsFile || golawErrorsFile(t.file.Path, t.src) {
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

// golawErrorsFile reports a golaw generated errors file: the name alone is
// not enough, because a hand written file of that name is still judged.
func golawErrorsFile(p string, src []byte) bool {
	base := path.Base(p)
	if strings.HasSuffix(base, "_test.go") {
		return false
	}
	named := base == golawErrorsName+".go" ||
		(strings.HasPrefix(base, golawErrorsName+"_") && strings.HasSuffix(base, ".go"))
	_, golaw := genheader.Parse(src)
	return named && golaw
}
