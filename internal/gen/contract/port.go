package contract

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"path"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/model"
)

// Reader implements genkind.Contracts over a snapshot with go/parser only, so
// the hook path reads a contract the agent just edited without the go
// command and without loading the package.
type Reader struct{ Snapshot model.Snapshot }

// Port reads the interface symbol of the contract file at rel. A shape that
// needs types answers the refusal Ports gave it, and a symbol the file does
// not declare as an interface answers ErrNoPort naming why.
func (r Reader) Port(rel, symbol string) (genkind.Port, error) {
	content, err := r.Snapshot.Read(rel)
	if err != nil {
		return genkind.Port{}, fmt.Errorf("read port contract %s: %w", rel, err)
	}
	ports, err := Ports(rel, content)
	if err != nil && !errors.Is(err, genkind.ErrCannotJudge) {
		return genkind.Port{}, err
	}
	for i := range ports {
		if ports[i].Name == symbol {
			return ports[i], nil
		}
	}
	if refusal, ok := refusalOf(err, symbol); ok {
		return genkind.Port{}, refusal
	}
	if declares(rel, content, symbol) {
		return genkind.Port{}, fmt.Errorf("%w: %s in %s is not an interface", ErrNoPort, symbol, rel)
	}
	return genkind.Port{}, fmt.Errorf("%w: %s in %s", ErrNoPort, symbol, rel)
}

// Ports answers every interface type of one file in source order, for golaw
// map and for Port. A shape only types could settle is left out and joined
// into the error, so one generic interface never hides the ports beside it.
func Ports(rel string, content []byte) ([]genkind.Port, error) {
	file, fset, err := parse(rel, content)
	if err != nil {
		return nil, err
	}
	c := contractFile{rel: rel, fset: fset, content: content, file: file}
	var ports []genkind.Port
	var refusals []error
	for _, spec := range typeSpecs(file) {
		port, ok, err := c.judge(spec.gen, spec.spec)
		switch {
		case err != nil:
			refusals = append(refusals, err)
		case ok:
			ports = append(ports, port)
		}
	}
	return ports, errors.Join(refusals...)
}

// refusalError is a contract shape the reader cannot judge without types; it
// matches genkind.ErrCannotJudge so the command maps it to exit 2, and its
// text names the way out instead of a guess at the shape.
type refusalError struct {
	path, construct, symbol string
	line                    int
}

func (e refusalError) Error() string {
	return fmt.Sprintf("%s:%d %s in %s cannot be judged without types: mark the file // golaw:handwritten <reason> and write the port by hand",
		e.path, e.line, e.construct, e.symbol)
}

// Is lets errors.Is match the kind sentinel without wrapping a second error.
func (e refusalError) Is(target error) bool { return target == genkind.ErrCannotJudge }

// contractFile is one parsed contract, kept together so the helpers that
// judge a declaration take the declaration alone.
type contractFile struct {
	rel     string
	fset    *token.FileSet
	content []byte
	file    *ast.File
}

// judge answers the port a type declaration is, false for a type that is no
// interface, or the refusal of a shape that needs types. Embedded interfaces
// of the same package are refused too: telling one from a type set or a
// foreign interface needs the package, which a one file reader never loads.
func (c contractFile) judge(gen *ast.GenDecl, spec *ast.TypeSpec) (genkind.Port, bool, error) {
	iface, isInterface := spec.Type.(*ast.InterfaceType)
	switch {
	case spec.Assign.IsValid():
		return genkind.Port{}, false, c.refuse("alias", spec.Name.Name, spec.Name)
	case !isInterface:
		return genkind.Port{}, false, nil
	case spec.TypeParams != nil:
		return genkind.Port{}, false, c.refuse("generic interface", spec.Name.Name, spec.Name)
	}
	methods, err := c.methods(spec.Name.Name, iface)
	if err != nil {
		return genkind.Port{}, false, err
	}
	return genkind.Port{
		Name: spec.Name.Name, Package: c.file.Name.Name, Path: c.rel, Dir: path.Dir(c.rel),
		Doc: portDoc(gen, spec), Methods: methods, Imports: imports(c.file), Line: c.line(spec.Name),
	}, true, nil
}

// methods answers the methods of an interface in source order, or the
// refusal of its first field that is no method.
func (c contractFile) methods(symbol string, iface *ast.InterfaceType) ([]genkind.Method, error) {
	var out []genkind.Method
	for _, field := range iface.Methods.List {
		fn, isFunc := field.Type.(*ast.FuncType)
		if len(field.Names) == 0 || !isFunc {
			return nil, c.refuse(embeddedConstruct(c, field.Type), symbol, field)
		}
		out = append(out, genkind.Method{
			Name: field.Names[0].Name, Doc: doc(field.Doc), Line: c.line(field.Names[0]),
			Params: params(c.fset, c.content, fn.Params), Results: params(c.fset, c.content, fn.Results),
		})
	}
	return out, nil
}

func (c contractFile) refuse(construct, symbol string, at ast.Node) error {
	return refusalError{path: c.rel, construct: construct, symbol: symbol, line: c.line(at)}
}

func (c contractFile) line(node ast.Node) int { return c.fset.Position(node.Pos()).Line }

// embeddedConstruct names an interface element that is no method: a union or
// a ~T term is a type set, anything else an embedded type written as is.
func embeddedConstruct(c contractFile, typ ast.Expr) string {
	switch t := typ.(type) {
	case *ast.BinaryExpr:
		return "type set"
	case *ast.UnaryExpr:
		if t.Op == token.TILDE {
			return "type set"
		}
	}
	return "embedded " + source(c.fset, c.content, typ)
}

// portDoc is the spec's own doc, else the declaration's when the declaration
// holds this spec alone, the way go/doc attaches it.
func portDoc(gen *ast.GenDecl, spec *ast.TypeSpec) string {
	if spec.Doc != nil || len(gen.Specs) != 1 {
		return doc(spec.Doc)
	}
	return doc(gen.Doc)
}

type typeSpec struct {
	gen  *ast.GenDecl
	spec *ast.TypeSpec
}

// typeSpecs answers every type spec of the file with its declaration, in
// source order.
func typeSpecs(file *ast.File) []typeSpec {
	var out []typeSpec
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			if ts, isType := spec.(*ast.TypeSpec); isType {
				out = append(out, typeSpec{gen: gen, spec: ts})
			}
		}
	}
	return out
}

// refusalOf finds the refusal of symbol among the joined refusals of Ports.
func refusalOf(err error, symbol string) (refusalError, bool) {
	var joined interface{ Unwrap() []error }
	if !errors.As(err, &joined) {
		return refusalError{}, false
	}
	for _, each := range joined.Unwrap() {
		var refusal refusalError
		if errors.As(each, &refusal) && refusal.symbol == symbol {
			return refusal, true
		}
	}
	return refusalError{}, false
}

// declares reports whether the file declares a type named symbol, so a port
// request on a struct says it is not an interface instead of not found.
func declares(rel string, content []byte, symbol string) bool {
	file, _, err := parse(rel, content)
	if err != nil {
		return false
	}
	for _, spec := range typeSpecs(file) {
		if spec.spec.Name.Name == symbol {
			return true
		}
	}
	return false
}
