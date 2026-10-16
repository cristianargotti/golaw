package goast

import (
	"go/ast"
	"go/token"

	"github.com/cristianargotti/golaw/internal/model"
)

const codeEmptyBody = "empty-body"

// SonarEmptyBody is goast.sonar-empty-body (Sonar rule S1186): a function or
// a function literal whose body holds no statement and no comment, inside
// the braces or on the closing line, is a finding.
func SonarEmptyBody() model.Check {
	return check{spec: spec("sonar-empty-body"), judge: judgeEmptyBody}
}

func judgeEmptyBody(t *target, _ *state) ([]model.Finding, error) {
	file, fset, err := t.parsed()
	if err != nil {
		return nil, err
	}
	var out []model.Finding
	ast.Inspect(file, func(n ast.Node) bool {
		body, name := functionBody(n)
		if body == nil || len(body.List) > 0 || hasComment(file, fset, body) {
			return true
		}
		message := name + " has an empty body with no comment saying why: say why inside the braces or on the closing line"
		out = append(out, t.finding(codeEmptyBody, body.Lbrace, message))
		return true
	})
	return out, nil
}

// functionBody gives the name the message uses, and nil for a declaration
// without a body, whose code lives elsewhere and cannot be empty.
func functionBody(n ast.Node) (*ast.BlockStmt, string) {
	switch fn := n.(type) {
	case *ast.FuncDecl:
		return fn.Body, "function " + fn.Name.Name
	case *ast.FuncLit:
		return fn.Body, "a function literal"
	default:
		return nil, ""
	}
}

// hasComment takes the closing line too, so a one-line body such as
// func() {} // why passes like a comment inside the braces.
func hasComment(file *ast.File, fset *token.FileSet, body *ast.BlockStmt) bool {
	closing := fset.Position(body.Rbrace).Line
	for _, group := range file.Comments {
		inside := group.Pos() > body.Lbrace && group.Pos() < body.Rbrace
		if inside || fset.Position(group.Pos()).Line == closing {
			return true
		}
	}
	return false
}
