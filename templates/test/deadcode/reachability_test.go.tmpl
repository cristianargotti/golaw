//go:build deadcode

package deadcode_test

import (
	"cmp"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestModuleRequirementsHaveConsumers keeps unused dependencies out of the graph.
func TestModuleRequirementsHaveConsumers(t *testing.T) {
	t.Parallel()
	command := exec.CommandContext(t.Context(), "go", "mod", "tidy", "-diff")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(command.Environ(), "GOTOOLCHAIN=local")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("module graph requires an explicit tidy change: %s: %v", output, err)
	}
}

// TestProductionExportsHaveConsumers avoids reachability claims because name matching admits possible consumers.
func TestProductionExportsHaveConsumers(t *testing.T) {
	t.Parallel()
	symbols, references, err := productionSymbols(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("cannot measure production consumers: %v", err)
	}
	if references[""] {
		t.Logf("dynamic method lookup: the generated query methods are not judged")
	}
	for _, symbol := range symbols {
		if !references[symbol.name] {
			t.Errorf("%s:%d: %s: add a production consumer or remove the symbol", symbol.path, symbol.line, symbol.name)
		}
	}
}

type productionSymbol struct {
	name, path string
	line       int
}

// Ordinary methods may have implicit consumers and a generator restores what it wrote, so only sqlc query methods are candidates.
func productionSymbols(root string) ([]productionSymbol, map[string]bool, error) {
	set := token.NewFileSet()
	module, moduleErr := productionModule(root)
	files, readErr := productionFiles(root, set)
	if err := errors.Join(moduleErr, readErr); err != nil {
		return nil, nil, err
	}
	references := productionFacts(files, module)
	var symbols []productionSymbol
	for name, file := range files {
		if pkg := path.Join(module, path.Dir(name)); !strings.HasPrefix(name, "internal/") || strings.HasSuffix(name, "_test.go") || references["test:"+pkg] && !references["import:"+pkg] {
			continue
		}
		for _, declaration := range file.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok && function.Name.IsExported() && (function.Recv == nil && !ast.IsGenerated(file) || function.Recv != nil && !references[""] && references["query:"+name] && references["type:"+path.Join(path.Dir(name), strings.Trim(strings.SplitN(types.ExprString(function.Recv.List[0].Type), "[", 2)[0], "*()"))]) {
				symbols = append(symbols, productionSymbol{function.Name.Name, name, set.Position(function.Name.Pos()).Line})
			}
		}
	}
	slices.SortFunc(symbols, func(a, b productionSymbol) int {
		return cmp.Or(cmp.Compare(a.path, b.path), cmp.Compare(a.line, b.line), cmp.Compare(a.name, b.name))
	})
	return symbols, references, nil
}

func productionModule(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(strings.SplitN(line, "//", 2)[0])
		if err == nil && len(fields) >= 2 && fields[0] == "module" {
			return strings.Trim(fields[1], "\"`"), nil
		}
	}
	return "", fmt.Errorf("read module declaration: %w", errors.Join(err, fs.ErrInvalid))
}

func productionFiles(root string, set *token.FileSet) (map[string]*ast.File, error) {
	tree := os.DirFS(root)
	files := make(map[string]*ast.File)
	return files, errors.Join(fs.WalkDir(tree, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %s: %w", name, walkErr)
		}
		if entry.IsDir() && name != "." {
			return productionDirectory(tree, name)
		}
		if base := path.Base(name); path.Ext(name) != ".go" || strings.HasPrefix(base, ".") || strings.HasPrefix(base, "_") {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s: symlinked Go source: %w", name, fs.ErrInvalid)
		}
		mode := parser.ParseComments | parser.SkipObjectResolution
		if strings.HasSuffix(name, "_test.go") {
			mode = parser.ImportsOnly | parser.SkipObjectResolution
		}
		var err error
		files[name], err = parser.ParseFile(set, filepath.Join(root, filepath.FromSlash(name)), nil, mode)
		return errors.Join(err)
	}))
}

func productionDirectory(tree fs.FS, name string) error {
	base := path.Base(name)
	if strings.HasPrefix(base, ".") || base == "testdata" || base == "fixtures" || base == "vendor" || name == "test/deadcode" {
		return fs.SkipDir
	}
	if _, err := fs.Stat(tree, path.Join(name, "go.mod")); err == nil {
		return fs.SkipDir
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect module %s: %w", name, err)
	}
	return nil
}

// Test-only imports from other directories identify packages that serve tests by design.
func productionFacts(files map[string]*ast.File, module string) map[string]bool {
	facts := make(map[string]bool)
	for name, file := range files {
		prefix := "test:"
		if !strings.HasSuffix(name, "_test.go") {
			prefix = "import:"
			header := func(match func(string) bool) bool {
				return slices.ContainsFunc(file.Comments, func(group *ast.CommentGroup) bool {
					return slices.ContainsFunc(group.List, func(comment *ast.Comment) bool { return comment.Pos() < file.Package && match(comment.Text) })
				})
			}
			generated := header(func(text string) bool { return text == "// Code generated by sqlc. DO NOT EDIT." })
			facts["query:"+name] = generated && header(func(text string) bool { return strings.HasPrefix(text, "// source: ") })
			productionReferences(file, path.Dir(name), generated, facts)
		}
		for _, spec := range file.Imports {
			if imported, err := strconv.Unquote(spec.Path.Value); err == nil && imported != path.Join(module, path.Dir(name)) {
				facts[prefix+imported] = true
			}
		}
	}
	facts[""] = facts[""] && facts["import:reflect"]
	return facts
}

// The empty reference key carries a method-only limit because no Go identifier is empty.
func productionReferences(file *ast.File, directory string, generated bool, facts map[string]bool) {
	var stack []ast.Node
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if name, ok := node.(*ast.Ident); ok && name != file.Name && !declarationName(name, stack[len(stack)-1]) {
			facts[name.Name] = true
		}
		if named, ok := node.(*ast.TypeSpec); ok && generated && len(stack) == 2 {
			facts["type:"+path.Join(directory, named.Name.Name)] = true
		}
		stack = append(stack, node)
		if selector, ok := node.(*ast.SelectorExpr); ok {
			call, invoked := stack[len(stack)-2].(*ast.CallExpr)
			facts[""] = facts[""] || selector.Sel.Name == "MethodByName" || selector.Sel.Name == "Method" && invoked && call.Fun == selector
		}
		return true
	})
}

// Declaration names cannot consume the symbols they introduce.
func declarationName(name *ast.Ident, parent ast.Node) bool {
	switch value := parent.(type) {
	case *ast.FuncDecl:
		return value.Name == name
	case *ast.TypeSpec, *ast.ImportSpec, *ast.LabeledStmt:
		return parent.Pos() == name.Pos()
	case *ast.ValueSpec:
		return slices.Contains(value.Names, name)
	case *ast.Field:
		return slices.Contains(value.Names, name)
	case *ast.AssignStmt:
		return value.Tok == token.DEFINE && slices.Contains(value.Lhs, ast.Expr(name))
	case *ast.RangeStmt:
		return value.Tok == token.DEFINE && (value.Key == name || value.Value == name)
	}
	return false
}
