package goast

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

const helperSource = `package a

import "os"

// note: a marker
var a = os.Getenv("X") // trailing: another
var b = f(1)
var c = x.y.Z()
var d = (g)()
var e = "quoted"
var f = 'r'
var g = 1
var h *T
var i pkg.T
var j []int

// bare:
// prose with bare: inside
/* block: says why */
/* empty: */
`

func parse(t *testing.T) (*ast.File, *token.FileSet) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "a.go", helperSource, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	return file, fset
}

// varSpec is the i-th package-level var declaration of the fixture.
func varSpec(t *testing.T, file *ast.File, i int) *ast.ValueSpec {
	t.Helper()
	gen, ok := file.Decls[i+1].(*ast.GenDecl)
	if !ok {
		t.Fatalf("declaration %d is not a var", i)
	}
	spec, ok := gen.Specs[0].(*ast.ValueSpec)
	if !ok {
		t.Fatalf("declaration %d has no value spec", i)
	}
	return spec
}

// value is the i-th package-level var initializer.
func value(t *testing.T, file *ast.File, i int) ast.Expr {
	t.Helper()
	return varSpec(t, file, i).Values[0]
}

// call is the i-th package-level var initializer as a call.
func call(t *testing.T, file *ast.File, i int) *ast.CallExpr {
	t.Helper()
	expr, ok := value(t, file, i).(*ast.CallExpr)
	if !ok {
		t.Fatalf("value %d is not a call", i)
	}
	return expr
}

// varType is the i-th package-level var type.
func varType(t *testing.T, file *ast.File, i int) ast.Expr {
	t.Helper()
	return varSpec(t, file, i).Type
}

func TestSelectorAndCallee(t *testing.T) {
	t.Parallel()
	file, _ := parse(t)
	first := call(t, file, 0)
	if pkg, name, ok := selector(first.Fun); !ok || pkg != "os" || name != "Getenv" {
		t.Errorf("selector = %s, %s, %v", pkg, name, ok)
	}
	if _, _, ok := selector(first); ok {
		t.Error("a call is not a selector")
	}
	if _, _, ok := selector(call(t, file, 2).Fun); ok {
		t.Error("a selector on a selector has no plain identifier")
	}
	want := []string{"os.Getenv", "f", ".Z", ""}
	for i, name := range want {
		if got := callee(call(t, file, i)); got != name {
			t.Errorf("callee(%d) = %q, want %q", i, got, name)
		}
	}
	if lastSegment("a.b.C") != "C" || lastSegment("plain") != "plain" {
		t.Error("lastSegment is wrong")
	}
}

func TestTypeNameLiteralsAndComments(t *testing.T) {
	t.Parallel()
	file, fset := parse(t)
	if typeName(varType(t, file, 7)) != "T" || typeName(varType(t, file, 8)) != "T" || typeName(varType(t, file, 9)) != "" {
		t.Error("typeName must name identifiers, selectors and pointers only")
	}
	if typeName(ast.NewIdent("Plain")) != "Plain" {
		t.Error("typeName must name an identifier")
	}
	if s, ok := stringLiteral(value(t, file, 4)); !ok || s != "quoted" {
		t.Errorf("stringLiteral(quoted) = %q, %v", s, ok)
	}
	if _, ok := stringLiteral(value(t, file, 5)); ok {
		t.Error("a rune is not a string literal")
	}
	if _, ok := stringLiteral(value(t, file, 1)); ok {
		t.Error("a call is not a string literal")
	}
	if _, ok := stringLiteral(&ast.BasicLit{Kind: token.STRING, Value: `"broken`}); ok {
		t.Error("an unterminated literal does not unquote")
	}
	marked := commentLines(file, fset, "note:")
	if len(marked) != 1 || !marked[5] {
		t.Errorf("commentLines(note) = %v", marked)
	}
	if trailing := commentLines(file, fset, "trailing:"); !trailing[6] {
		t.Errorf("commentLines(trailing) = %v", trailing)
	}
}

func TestCommentLinesWantTheMarkerFirstAndText(t *testing.T) {
	t.Parallel()
	file, fset := parse(t)
	if block := commentLines(file, fset, "block:"); len(block) != 1 || !block[19] {
		t.Errorf("commentLines(block) = %v", block)
	}
	for _, marker := range []string{"marker", "bare:", "empty:"} {
		if lines := commentLines(file, fset, marker); len(lines) != 0 {
			t.Errorf("a marker inside prose or with no text clears nothing: commentLines(%s) = %v", marker, lines)
		}
	}
}
