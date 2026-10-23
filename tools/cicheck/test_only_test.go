package cicheck_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestDoctorAndProofKeepTestOnlyFunctionsInTests holds the unexported
// functions of the doctor and proof packages to the rule D1 holds their
// exported ones to, by name: a function no production file of its package
// names serves the tests alone and belongs in a test file, because
// unexporting it moves it out of the deadcode rule without removing it.
func TestDoctorAndProofKeepTestOnlyFunctionsInTests(t *testing.T) {
	t.Parallel()
	for _, dir := range []string{"internal/doctor", "internal/proof"} {
		for _, name := range testOnlyFunctions(t, dir) {
			t.Errorf("%s: %s: only tests call it; move it into a test file", dir, name)
		}
	}
}

// testOnlyFunctions are the unexported functions the production files of a
// directory declare and never name elsewhere; a name declared in two files
// of different build constraints counts each declaration apart.
func testOnlyFunctions(t *testing.T, dir string) []string {
	t.Helper()
	declared, idents := map[string]int{}, map[string]int{}
	var order []string
	for _, file := range packageSources(t, dir) {
		for _, name := range unexportedFunctions(file) {
			if declared[name] == 0 {
				order = append(order, name)
			}
			declared[name]++
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if ident, ok := node.(*ast.Ident); ok {
				idents[ident.Name]++
			}
			return true
		})
	}
	return slices.DeleteFunc(order, func(name string) bool { return idents[name] > declared[name] })
}

// packageSources are the parsed production Go files of a directory.
func packageSources(t *testing.T, dir string) []*ast.File {
	t.Helper()
	root := filepath.Join("..", "..", filepath.FromSlash(dir))
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	set := token.NewFileSet()
	var files []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(set, filepath.Join(root, name), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		files = append(files, file)
	}
	return files
}

// unexportedFunctions are the unexported functions a file declares, never
// a method.
func unexportedFunctions(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		if function, ok := decl.(*ast.FuncDecl); ok && function.Recv == nil && !function.Name.IsExported() {
			names = append(names, function.Name.Name)
		}
	}
	return names
}
