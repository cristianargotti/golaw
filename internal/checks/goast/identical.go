package goast

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
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
	file, _, err := t.parsed()
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
		shape := functionShape(fn)
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

// functionShape walks the receiver type, the type parameters, the
// parameters and the body of a function into one key: every node opens
// with its type and the text its children do not carry and closes after
// them, and no position or comment enters it, so two functions that
// differ only in name, result names, layout or comments give one key.
func functionShape(fn *ast.FuncDecl) string {
	roots := []ast.Node{&ast.FuncType{TypeParams: fn.Type.TypeParams, Params: fn.Type.Params}, fn.Body}
	if fn.Recv != nil {
		roots = append(roots, fn.Recv.List[0].Type)
	}
	var words []string
	for _, root := range roots {
		ast.Inspect(root, func(n ast.Node) bool {
			switch n.(type) {
			case nil:
				words = append(words, ")")
				return false
			case *ast.CommentGroup:
				return false
			}
			words = append(words, fmt.Sprintf("(%T %q", n, nodeText(n)))
			return true
		})
	}
	return strings.Join(words, " ")
}

// nodeText is what sets a node apart beyond its type and its children: the
// name of an identifier, the spelling of a literal, the operator or the
// keyword token, and the layout of nodeLayout.
func nodeText(n ast.Node) string {
	switch n := n.(type) {
	case *ast.Ident:
		return n.Name
	case *ast.BasicLit:
		return n.Value
	}
	if tok := nodeToken(n); tok != token.ILLEGAL {
		return tok.String() + nodeLayout(n)
	}
	return nodeLayout(n)
}

// nodeToken is the operator or the keyword a node holds outside its
// children, ILLEGAL for a node that holds none.
func nodeToken(n ast.Node) token.Token {
	switch n := n.(type) {
	case *ast.BinaryExpr:
		return n.Op
	case *ast.UnaryExpr:
		return n.Op
	case *ast.AssignStmt:
		return n.Tok
	case *ast.IncDecStmt:
		return n.Tok
	case *ast.BranchStmt:
		return n.Tok
	case *ast.RangeStmt:
		return n.Tok
	case *ast.GenDecl:
		return n.Tok
	default:
		return token.ILLEGAL
	}
}

// nodeLayout says which optional children a node has and where a list of
// children splits, wherever two different trees would otherwise walk
// alike: the clauses of a for, the indices of a slice, the type of a
// composite literal, the communication of a select case, the left side of
// an assignment, the names and the type of a value spec; and the parts no
// child holds: the direction of a channel, the ellipsis of a call and the
// equal sign of an alias.
func nodeLayout(n ast.Node) string {
	switch n := n.(type) {
	case *ast.ForStmt:
		return present(n.Init != nil, n.Cond != nil, n.Post != nil)
	case *ast.SliceExpr:
		return present(n.Low != nil, n.High != nil, n.Max != nil)
	case *ast.CompositeLit:
		return present(n.Type != nil)
	case *ast.CommClause:
		return present(n.Comm != nil)
	case *ast.AssignStmt:
		return strconv.Itoa(len(n.Lhs))
	case *ast.ValueSpec:
		return strconv.Itoa(len(n.Names)) + present(n.Type != nil)
	case *ast.ChanType:
		return strconv.Itoa(int(n.Dir))
	case *ast.CallExpr:
		return present(n.Ellipsis.IsValid())
	case *ast.TypeSpec:
		return present(n.Assign.IsValid())
	default:
		return ""
	}
}

// present writes one digit per optional part, 1 when it is there.
func present(parts ...bool) string {
	out := make([]byte, len(parts))
	for i, part := range parts {
		out[i] = '0'
		if part {
			out[i] = '1'
		}
	}
	return string(out)
}
