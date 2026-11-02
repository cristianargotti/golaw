package port

import (
	"cmp"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"unicode"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/model"
)

// The owned parts of a port: the cases file of its test package, and the
// stub of an interface the area does not declare yet.
const (
	casesFile     = "cases.go"
	casesSuffix   = "Cases"
	stubsTemplate = "ports.go"
)

// ownedPart is what a run asks of the owned code: whole files that are
// gone, appends for the methods that are new, decisions for what is left.
type ownedPart struct {
	Owned     []genkind.File
	Appends   []genkind.Append
	Decisions []genkind.Decision
}

// casesPath is the owned cases file of a port's test package.
func casesPath(p genkind.Port) string {
	return testDir(p) + "/" + casesFile
}

// newCases renders the whole cases file, one hole per method.
func newCases(in genkind.Input, p genkind.Port, v view) (genkind.File, error) {
	content, err := renderFile(in, casesFile, v, false, nil)
	if err != nil {
		return genkind.File{}, err
	}
	return genkind.File{Path: casesPath(p), Content: content}, nil
}

// stubPart writes the interface stub of a port the area does not declare:
// appended to an existing file, so the agent's code is never rewritten, or
// a new owned file that carries the package doc when no file of the area
// has one yet, since the use case files a run writes beside it carry none.
func stubPart(in genkind.Input, c contractFile, v view) (ownedPart, error) {
	v.Doc = c.Undocumented
	content, err := renderFile(in, stubsTemplate, v, false, nil)
	if err != nil {
		return ownedPart{}, err
	}
	if !c.Exists {
		return ownedPart{Owned: []genkind.File{{Path: c.Path, Content: content}}}, nil
	}
	decls, err := declarations(content)
	if err != nil {
		return ownedPart{}, err
	}
	return ownedPart{Appends: []genkind.Append{{Path: c.Path, Decls: decls, Holes: []string{v.Hole}}}}, nil
}

// regenerateCases appends the cases function of every new method and names
// each function left by a method that is gone; a deleted cases file is
// written again, since the regenerable Cases calls its functions.
func regenerateCases(in genkind.Input, p genkind.Port, v view) (ownedPart, error) {
	content, err := in.Snapshot.Read(casesPath(p))
	if errors.Is(err, model.ErrNotInSnapshot) {
		return wholeCases(in, p, v)
	}
	if err != nil {
		return ownedPart{}, fmt.Errorf("read the cases of port %s.%s: %w", v.Area, v.Port, err)
	}
	funcs, err := caseFuncs(content)
	if err != nil {
		return ownedPart{}, fmt.Errorf("%w: %s does not parse: %w", genkind.ErrCannotJudge, casesPath(p), err)
	}
	var part ownedPart
	missing := v
	missing.Methods = nil
	for i := range v.Methods {
		m := &v.Methods[i]
		if _, ok := funcs[m.Cases]; !ok {
			missing.Methods = append(missing.Methods, *m)
		}
		delete(funcs, m.Cases)
	}
	if len(missing.Methods) > 0 {
		appended, err := appendCases(in, p, missing)
		if err != nil {
			return ownedPart{}, err
		}
		part.Appends = []genkind.Append{appended}
	}
	part.Decisions = orphans(p, funcs)
	return part, nil
}

// wholeCases writes a cases file the snapshot no longer holds.
func wholeCases(in genkind.Input, p genkind.Port, v view) (ownedPart, error) {
	file, err := newCases(in, p, v)
	if err != nil {
		return ownedPart{}, err
	}
	return ownedPart{Owned: []genkind.File{file}}, nil
}

// appendCases renders the cases functions of the methods the file lacks.
func appendCases(in genkind.Input, p genkind.Port, v view) (genkind.Append, error) {
	content, err := renderFile(in, casesFile, v, false, nil)
	if err != nil {
		return genkind.Append{}, err
	}
	decls, err := declarations(content)
	if err != nil {
		return genkind.Append{}, err
	}
	ids := make([]string, 0, len(v.Methods))
	for i := range v.Methods {
		ids = append(ids, v.Methods[i].Hole)
	}
	return genkind.Append{Path: casesPath(p), Decls: decls, Holes: ids}, nil
}

// orphans names each cases function whose method left the port, sorted by
// line; deleting owned code is the agent's decision (M7.9.4).
func orphans(p genkind.Port, funcs map[string]int) []genkind.Decision {
	var out []genkind.Decision
	for name, line := range funcs {
		base, ok := strings.CutSuffix(name, casesSuffix)
		if !ok || base == "" {
			continue
		}
		runes := []rune(base)
		runes[0] = unicode.ToUpper(runes[0])
		out = append(out, genkind.Decision{Path: casesPath(p), Line: line, Text: fmt.Sprintf(
			"%s drives %s, which is no longer in port %s: delete it or restore the method in %s", name, string(runes), p.Name, p.Path)})
	}
	slices.SortFunc(out, func(a, b genkind.Decision) int { return cmp.Compare(a.Line, b.Line) })
	return out
}

// caseFuncs maps each top level function of a cases file to the line of
// its func keyword.
func caseFuncs(content []byte) (map[string]int, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, casesFile, content, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", casesFile, err)
	}
	out := map[string]int{}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
			out[fn.Name.Name] = fset.Position(fn.Pos()).Line
		}
	}
	return out, nil
}

// declarations answers each top level declaration of a rendered file from
// its doc comment to its end, so the engine appends complete, formatted
// declarations and never a fragment.
func declarations(content []byte) ([][]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", content, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("%w: a rendered declaration does not parse: %w", genkind.ErrCannotJudge, err)
	}
	tf := fset.File(file.Pos())
	out := make([][]byte, 0, len(file.Decls))
	for _, decl := range file.Decls {
		start := decl.Pos()
		if doc := docOf(decl); doc != nil {
			start = doc.Pos()
		}
		out = append(out, slices.Clone(content[tf.Offset(start):tf.Offset(decl.End())]))
	}
	return out, nil
}

// docOf answers the doc comment of a function or a general declaration.
func docOf(decl ast.Decl) *ast.CommentGroup {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		return d.Doc
	case *ast.GenDecl:
		return d.Doc
	}
	return nil
}
