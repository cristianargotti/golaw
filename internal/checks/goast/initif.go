package goast

import (
	"go/ast"
	"go/token"
	"regexp"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	codeInitIfErrNil = "init-if-err-nil"
	fallbackMarker   = "fallback:"
)

// parseIdiom names the calls whose failure is answered by a fallback by
// design: parsing, decoding, reading a file and the constructors with a
// default.
var parseIdiom = regexp.MustCompile(`^(Parse\w*|Sscan\w*|Unmarshal\w*|Decode\w*|Atoi|ReadFile|New(V\d|Random|UUID)|From\w+)$`)

// InitIfErrNil is goast.init-if-err-nil: an init-if that keeps the happy
// path on err == nil drops the error; a parse or decode idiom and a line
// marked fallback: are the accepted shapes.
func InitIfErrNil() model.Check {
	return check{spec: spec("init-if-err-nil"), judge: judgeInitIf}
}

func judgeInitIf(t *target, _ *state) ([]model.Finding, error) {
	file, fset, err := t.parsed()
	if err != nil {
		return nil, err
	}
	marked := commentLines(file, fset, fallbackMarker)
	var out []model.Finding
	ast.Inspect(file, func(n ast.Node) bool {
		stmt, ok := n.(*ast.IfStmt)
		if !ok || !keepsHappyPath(stmt) || marked[fset.Position(stmt.Pos()).Line] {
			return true
		}
		message := "an init-if keeps the happy path and drops the error: handle it," +
			" or mark the line with a fallback: comment saying why the default applies"
		out = append(out, t.finding(codeInitIfErrNil, stmt.Pos(), message))
		return true
	})
	return out, nil
}

// keepsHappyPath reports if x, err := call(); err == nil over a call that
// is not a parse idiom.
func keepsHappyPath(stmt *ast.IfStmt) bool {
	init, ok := stmt.Init.(*ast.AssignStmt)
	if !ok || init.Tok != token.DEFINE || len(init.Rhs) != 1 {
		return false
	}
	cond, ok := stmt.Cond.(*ast.BinaryExpr)
	if !ok || cond.Op != token.EQL || !isIdent(cond.X, "err") || !isIdent(cond.Y, "nil") {
		return false
	}
	call, ok := init.Rhs[0].(*ast.CallExpr)
	return !ok || !parseIdiom.MatchString(lastSegment(callee(call)))
}

// isIdent reports an identifier with the name.
func isIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}
