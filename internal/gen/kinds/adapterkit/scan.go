package adapterkit

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/model"
)

// Declared is where a function or method of the adapter's package lives.
type Declared struct {
	Path string
	Line int
}

// Package is what the adapter's directory already declares, read so a run
// appends only the stubs owned code lacks and names what lost its contract.
type Package struct {
	Methods map[string]Declared // methods of the adapter type in non-test files
	Funcs   map[string]Declared // top-level functions of non-test files
	Doc     bool                // a non-test file holds the package doc comment
}

// parsedFile is one non-test Go file of a directory with its positions.
type parsedFile struct {
	path string
	fset *token.FileSet
	file *ast.File
}

// goFiles parses the non-test Go files directly in dir; test files are left
// out because a helper there is never the adapter's code.
func goFiles(s model.Snapshot, dir string) ([]parsedFile, error) {
	var out []parsedFile
	for _, f := range s.Files() {
		if path.Dir(f.Path) != dir || !strings.HasSuffix(f.Path, ".go") || strings.HasSuffix(f.Path, "_test.go") {
			continue
		}
		content, err := s.Read(f.Path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f.Path, err)
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, f.Path, content, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("%w: %s does not parse: %w", genkind.ErrCannotJudge, f.Path, err)
		}
		out = append(out, parsedFile{path: f.Path, fset: fset, file: file})
	}
	return out, nil
}

// Scan reads the functions and the methods of typ declared in dir.
func Scan(s model.Snapshot, dir, typ string) (Package, error) {
	files, err := goFiles(s, dir)
	if err != nil {
		return Package{}, err
	}
	pkg := Package{Methods: map[string]Declared{}, Funcs: map[string]Declared{}}
	for _, f := range files {
		pkg.Doc = pkg.Doc || f.file.Doc != nil
		for _, decl := range f.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			at := Declared{Path: f.path, Line: f.fset.Position(fn.Name.Pos()).Line}
			switch {
			case fn.Recv == nil:
				pkg.Funcs[fn.Name.Name] = at
			case receiverType(fn.Recv) == typ:
				pkg.Methods[fn.Name.Name] = at
			}
		}
	}
	return pkg, nil
}

// receiverType is the base type name of a receiver, the star removed.
func receiverType(recv *ast.FieldList) string {
	if len(recv.List) == 0 {
		return ""
	}
	typ := recv.List[0].Type
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}
	if id, ok := typ.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// Sentinels answers, sorted and qualified, the error variables of the port's
// package whose name holds word, so a hole names the sentinel to answer.
func Sentinels(s model.Snapshot, port genkind.Port, word string) ([]string, error) {
	files, err := goFiles(s, port.Dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, f := range files {
		out = append(out, errorVars(f.file, port.Package, word)...)
	}
	slices.Sort(out)
	return out, nil
}

// errorVars answers the top-level Err variables of one file that hold word.
func errorVars(file *ast.File, pkg, word string) []string {
	var out []string
	for _, name := range varNames(file) {
		if strings.HasPrefix(name, "Err") && strings.Contains(name, word) {
			out = append(out, pkg+"."+name)
		}
	}
	return out
}

// varNames answers the names of the top-level variables of one file.
func varNames(file *ast.File) []string {
	var out []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			if value, isValue := spec.(*ast.ValueSpec); isValue {
				for _, name := range value.Names {
					out = append(out, name.Name)
				}
			}
		}
	}
	return out
}

// Sentence names the sentinels in a hole's constraint, or where the one the
// port lacks belongs, so the agent never invents a second one.
func Sentence(sentinels []string, dir string) string {
	switch len(sentinels) {
	case 0:
		return "the port's not found sentinel, declared once in " + dir + "/errors.go"
	case 1:
		return sentinels[0]
	}
	return "one of " + strings.Join(sentinels, ", ")
}

// Reconcile answers the port methods the package lacks, in port order, and
// one decision per exported method of owned code the port no longer
// declares (M7.9.4): owned code is the agent's, so the run never deletes it.
func Reconcile(t Target, methods []Method, pkg Package, owned []string) ([]Method, []genkind.Decision) {
	var missing []Method
	for i := range methods {
		if _, ok := pkg.Methods[methods[i].Name]; !ok {
			missing = append(missing, methods[i])
		}
	}
	var decisions []genkind.Decision
	for name, at := range pkg.Methods {
		if !token.IsExported(name) || !slices.Contains(owned, at.Path) || inPort(t.Port, name) {
			continue
		}
		decisions = append(decisions, genkind.Decision{
			Path: at.Path, Line: at.Line,
			Text: fmt.Sprintf("(*%s).%s is no longer in port %s: delete it or restore the method in %s",
				t.Name.Type, name, t.Port.Name, t.Port.Path),
		})
	}
	slices.SortFunc(decisions, func(a, b genkind.Decision) int {
		return cmp.Or(strings.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line))
	})
	return missing, decisions
}

func inPort(port genkind.Port, name string) bool {
	return slices.ContainsFunc(port.Methods, func(m genkind.Method) bool { return m.Name == name })
}
