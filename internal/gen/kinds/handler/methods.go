package handler

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/model"
)

// method is an operation method an owned file declares, where it starts.
type method struct {
	Name, Path string
	Line       int
}

// present is what the owned files of one entry declare now: the operation
// methods by name, the test functions by name, the operation methods of
// each code file and the owned files that exist.
type present struct {
	Methods map[string]method
	Tests   map[string]bool
	Count   map[string]int
	Files   map[string]bool
}

// scan reads the owned files of one entry; a deleted owned file is simply
// absent, and one that does not parse cannot be judged, because a guess
// would append a stub the agent already wrote.
func scan(s model.Snapshot, ownedPaths []string) (present, error) {
	p := present{Methods: map[string]method{}, Tests: map[string]bool{}, Count: map[string]int{}, Files: map[string]bool{}}
	for _, file := range ownedPaths {
		if s.Class(file) == model.ClassUnknown {
			continue
		}
		content, err := s.Read(file)
		if err != nil {
			return present{}, fmt.Errorf("handler owned file %s: %w", file, err)
		}
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, content, parser.SkipObjectResolution)
		if err != nil {
			return present{}, fmt.Errorf("%w: owned file %s does not parse: %w", genkind.ErrCannotJudge, file, err)
		}
		p.Files[file] = true
		p.record(fset, file, parsed)
	}
	return p, nil
}

func (p present) record(fset *token.FileSet, file string, parsed *ast.File) {
	test := strings.HasSuffix(file, "_test.go")
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		switch {
		case !ok:
		case test:
			p.Tests[fn.Name.Name] = true
		case operationShaped(fn):
			p.Methods[fn.Name.Name] = method{Name: fn.Name.Name, Path: file, Line: fset.Position(fn.Pos()).Line}
			p.Count[file]++
		}
	}
}

// operationShaped is the certainty rule: a method of *Handler, exported,
// whose second of exactly two parameters is <pkg>.<Name>RequestObject.
// Anything else is left alone, so a helper an agent wrote never becomes a
// decision.
func operationShaped(fn *ast.FuncDecl) bool {
	if fn.Recv == nil || len(fn.Recv.List) != 1 || !fn.Name.IsExported() {
		return false
	}
	star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	if id, isIdent := star.X.(*ast.Ident); !isIdent || id.Name != handlerType {
		return false
	}
	params := paramTypes(fn.Type.Params)
	if len(params) != 2 {
		return false
	}
	sel, ok := params[1].(*ast.SelectorExpr)
	return ok && sel.Sel.Name == fn.Name.Name+"RequestObject"
}

// paramTypes spells one type per parameter, so (a, b T) counts as two.
func paramTypes(list *ast.FieldList) []ast.Expr {
	var out []ast.Expr
	for _, field := range list.List {
		for range max(1, len(field.Names)) {
			out = append(out, field.Type)
		}
	}
	return out
}

// declsOf cuts each function of src from its doc comment to its closing
// brace, the bytes an append adds.
func declsOf(src []byte) ([][]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("handler stub: %w", err)
	}
	var out [][]byte
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		start := fn.Pos()
		if fn.Doc != nil {
			start = fn.Doc.Pos()
		}
		out = append(out, slices.Clone(src[fset.Position(start).Offset:fset.Position(fn.End()).Offset]))
	}
	return out, nil
}

// stub renders the method and the red test of one operation through the
// templates of golaw new, so an appended stub is byte for byte what golaw
// new would have written; it answers the method, then the test.
func stub(v view, op operation) ([]byte, []byte, error) {
	files, err := renderAll(v.Module, operationSpecs(v, op.Stem, 1, []operation{op}), genheader.Header{})
	if err != nil {
		return nil, nil, err
	}
	decls := make([][]byte, 0, len(files))
	for _, f := range files {
		d, err := declsOf(f.Content)
		if err != nil {
			return nil, nil, err
		}
		if len(d) != 1 {
			return nil, nil, fmt.Errorf("%w: %s holds %d", errStub, f.Path, len(d))
		}
		decls = append(decls, d[0])
	}
	return decls[0], decls[1], nil
}
