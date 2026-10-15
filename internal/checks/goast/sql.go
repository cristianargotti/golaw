package goast

import (
	"go/ast"
	"go/token"
	"regexp"

	"github.com/cristianargotti/golaw/internal/model"
)

const codeSQLFromStrings = "sql-from-strings"

var (
	sqlStatement = regexp.MustCompile(`\b(SELECT|INSERT INTO|UPDATE|DELETE FROM)\b`)
	sqlClause    = regexp.MustCompile(`\b(WHERE|FROM|VALUES)\b`)
)

// SQLFromStrings is goast.sql-from-strings: when the repository generates
// its queries with sqlc, a query formatted or concatenated from strings is
// a finding. The check applies only when sqlc.yaml sits at the root.
func SQLFromStrings() model.Check {
	return check{spec: spec("sql-from-strings"), gate: requireFile("sqlc.yaml"), judge: judgeSQL}
}

func judgeSQL(t *target, _ *state) ([]model.Finding, error) {
	file, _, err := t.parsed()
	if err != nil {
		return nil, err
	}
	var out []model.Finding
	ast.Inspect(file, func(n ast.Node) bool {
		switch e := n.(type) {
		case *ast.CallExpr:
			if formatsQuery(e) {
				message := "a query is formatted from a string: queries live in db/queries and go through sqlc"
				out = append(out, t.finding(codeSQLFromStrings, e.Pos(), message))
			}
		case *ast.BinaryExpr:
			if e.Op == token.ADD && concatenatesQuery(e) {
				message := "a query is concatenated from strings: queries live in db/queries and go through sqlc"
				out = append(out, t.finding(codeSQLFromStrings, e.Pos(), message))
				return false
			}
		}
		return true
	})
	return out, nil
}

// formatsQuery reports a fmt.Sprintf or fmt.Sprint whose first argument is
// a statement literal.
func formatsQuery(call *ast.CallExpr) bool {
	name := callee(call)
	if (name != "fmt.Sprintf" && name != "fmt.Sprint") || len(call.Args) == 0 {
		return false
	}
	value, ok := stringLiteral(call.Args[0])
	return ok && sqlStatement.MatchString(value)
}

// concatenatesQuery reports a + chain that mixes a clause literal with a
// value that is not a literal.
func concatenatesQuery(expr *ast.BinaryExpr) bool {
	clause, dynamic := false, false
	for _, operand := range operands(expr) {
		value, ok := stringLiteral(operand)
		switch {
		case !ok:
			dynamic = true
		case sqlClause.MatchString(value):
			clause = true
		}
	}
	return clause && dynamic
}

// operands flattens a left-nested + chain into its operands in order.
func operands(expr ast.Expr) []ast.Expr {
	if bin, ok := expr.(*ast.BinaryExpr); ok && bin.Op == token.ADD {
		return append(operands(bin.X), operands(bin.Y)...)
	}
	return []ast.Expr{expr}
}
