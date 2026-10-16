package goast

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	codeFunctionLength     = "function-length"
	codeFunctionStatements = "function-statements"
)

// FunctionLength is goast.function-length: a function has at most the
// function lines of the policy, counted the way funlen counts with comments
// ignored, and at most that many statements. Test files lose the limit.
func FunctionLength() model.Check {
	return check{spec: spec("function-length"), judge: judgeFunctionLength}
}

func judgeFunctionLength(t *target, s *state) ([]model.Finding, error) {
	file, fset, err := t.parsed()
	if err != nil {
		return nil, err
	}
	limit := s.in.Policy.Sizes.FunctionLines
	cmap := ast.NewCommentMap(fset, file, file.Comments)
	var out []model.Finding
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if n := countStatements(fn.Body.List); n > limit {
			message := fmt.Sprintf("function %s has %d statements, the limit is %d: give it one job", fn.Name.Name, n, limit)
			out = append(out, t.finding(codeFunctionStatements, fn.Name.Pos(), message))
			continue
		}
		if n := functionLines(fset, fn, cmap.Filter(fn)); n > limit {
			message := fmt.Sprintf("function %s is %d lines, the limit is %d: give it one job", fn.Name.Name, n, limit)
			out = append(out, t.finding(codeFunctionLength, fn.Name.Pos(), message))
		}
	}
	return out, nil
}

// functionLines counts the lines between the braces of a function minus
// the comment entries the comment map binds inside it, exactly as funlen
// does with ignore-comments: a multi-line block comment is one entry and a
// trailing comment on a code line is one too.
func functionLines(fset *token.FileSet, fn *ast.FuncDecl, cmap ast.CommentMap) int {
	start, end := fset.Position(fn.Pos()).Line, fset.Position(fn.End()).Line
	lines := end - start - 1
	for _, group := range cmap.Comments() {
		if fset.Position(group.Pos()).Line > start && fset.Position(group.End()).Line < end {
			lines -= len(group.List)
		}
	}
	return lines
}

// countStatements counts statements the way funlen does: every statement
// counts one, a block adds its statements, the six control statements add
// the statements of their body (an else branch is never descended), a case
// clause adds its body, and a function literal on the first right-hand side
// of an assignment, in a go or in a defer adds its statements.
func countStatements(list []ast.Stmt) int {
	total := 0
	for _, stmt := range list {
		total++
		switch s := stmt.(type) {
		case *ast.BlockStmt:
			total += countStatements(s.List) - 1
		case *ast.CaseClause:
			total += countStatements(s.Body)
		case *ast.AssignStmt:
			total += literalStatements(s.Rhs[0])
		case *ast.GoStmt:
			total += literalStatements(s.Call.Fun)
		case *ast.DeferStmt:
			total += literalStatements(s.Call.Fun)
		default:
			if body, ok := controlBody(stmt); ok {
				total += countStatements(body)
			}
		}
	}
	return total
}

// controlBody is the body list of the six control statements funlen
// descends into.
func controlBody(stmt ast.Stmt) ([]ast.Stmt, bool) {
	switch s := stmt.(type) {
	case *ast.ForStmt:
		return s.Body.List, true
	case *ast.RangeStmt:
		return s.Body.List, true
	case *ast.IfStmt:
		return s.Body.List, true
	case *ast.SwitchStmt:
		return s.Body.List, true
	case *ast.TypeSwitchStmt:
		return s.Body.List, true
	case *ast.SelectStmt:
		return s.Body.List, true
	default:
		return nil, false
	}
}

// literalStatements counts the statements of a function literal, zero for
// any other expression.
func literalStatements(expr ast.Expr) int {
	if lit, ok := expr.(*ast.FuncLit); ok {
		return countStatements(lit.Body.List)
	}
	return 0
}
