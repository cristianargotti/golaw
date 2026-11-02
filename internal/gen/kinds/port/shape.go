package port

import (
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
)

// maxParams is revive's argument-limit of the lint asset (A51): a fake of a
// longer method would carry the finding into the adopter's lint.
const maxParams = 7

const (
	contextType = "context.Context"
	errorType   = "error"
	blank       = "_"
	ctxParam    = "ctx"
)

// param is one parameter of a fake method under the name the fake uses.
type param struct {
	Name string
	Type typeExpr
}

// signature is one port method as the fake and the suite write it.
type signature struct {
	Name    string
	Line    int
	Context bool // the first parameter is a context.Context
	Failing bool // the last result is an error
	ctxName string
	Params  []param    // after the context
	Values  []typeExpr // the results before the error
	results []typeExpr
}

// shape reads one method of a port into the signature its fake declares.
// A method over the parameter limit is refused here, before any file is
// written, because the fake would only repeat the contract's finding.
func shape(p genkind.Port, m genkind.Method) (signature, error) {
	if len(m.Params) > maxParams {
		return signature{}, fmt.Errorf("%w: port %s.%s method %s at %s:%d takes %d parameters, over the %d of argument-limit: group them in one struct",
			genkind.ErrRequest, p.Package, p.Name, m.Name, p.Path, m.Line, len(m.Params), maxParams)
	}
	s := signature{Name: m.Name, Line: m.Line}
	reserved := reservedNames(p)
	params := m.Params
	if len(params) > 0 && params[0].Type == contextType {
		s.Context, s.ctxName, params = true, unshadowed(name(params[0].Name, ctxParam), reserved), params[1:]
	}
	offset := len(m.Params) - len(params)
	for i, mp := range params {
		typ, err := qualify(mp.Type, p.Package)
		if err != nil {
			return signature{}, fmt.Errorf("port %s.%s method %s at %s:%d: %w", p.Package, p.Name, m.Name, p.Path, m.Line, err)
		}
		n := unshadowed(name(mp.Name, "arg"+strconv.Itoa(i+offset+1)), reserved)
		s.Params = append(s.Params, param{Name: n, Type: typ})
	}
	results, err := qualifyResults(p, m)
	if err != nil {
		return signature{}, err
	}
	s.results, s.Values = results, results
	if n := len(results); n > 0 && results[n-1].Text == errorType {
		s.Failing, s.Values = true, results[:n-1]
	}
	if s.Context && !s.Failing {
		s.ctxName = blank
	}
	return s, nil
}

// qualifyResults reads every result type as the test package writes it.
func qualifyResults(p genkind.Port, m genkind.Method) ([]typeExpr, error) {
	out := make([]typeExpr, 0, len(m.Results))
	for _, r := range m.Results {
		typ, err := qualify(r.Type, p.Package)
		if err != nil {
			return nil, fmt.Errorf("port %s.%s method %s at %s:%d: %w", p.Package, p.Name, m.Name, p.Path, m.Line, err)
		}
		out = append(out, typ)
	}
	return out, nil
}

// name keeps a parameter's own name and gives an unnamed one a fallback.
func name(given, fallback string) string {
	if given == "" || given == blank {
		return fallback
	}
	return given
}

// unshadowed renames a parameter that would hide a name the body uses.
func unshadowed(n string, reserved map[string]bool) string {
	if reserved[n] {
		return n + "Arg"
	}
	return n
}

// reservedNames are the names a fake method body, the recording test or a
// contract import already uses, which a parameter of the same name would
// shadow.
func reservedNames(p genkind.Port) map[string]bool {
	reserved := map[string]bool{
		"f": true, "r": true, "err": true, "fake": true, "t": true, p.Package: true,
		"porttest": true, holes.SupportName: true, "errors": true, slicesPkg: true, path.Base(testDir(p)): true,
	}
	for _, imp := range p.Imports {
		reserved[importName(imp)] = true
	}
	return reserved
}

// importName is the name an import is referred to by in the contract file.
func importName(imp genkind.Import) string {
	if imp.Name != "" {
		return imp.Name
	}
	return path.Base(imp.Path)
}

// declaration spells the parameters and results of the fake method, with
// adjacent parameters of one type grouped as gofumpt's group-params wants.
func (s signature) declaration() string {
	all := s.Params
	if s.Context {
		all = append([]param{{Name: s.ctxName, Type: typeExpr{Text: contextType}}}, s.Params...)
	}
	var groups []string
	for i, p := range all {
		if i+1 < len(all) && all[i+1].Type.Text == p.Type.Text {
			groups = append(groups, p.Name+",")
			continue
		}
		groups = append(groups, p.Name+" "+p.Type.Text+",")
	}
	params := strings.TrimSuffix(strings.Join(groups, " "), ",")
	switch len(s.results) {
	case 0:
		return "(" + params + ")"
	case 1:
		return "(" + params + ") " + s.results[0].Text
	}
	texts := make([]string, 0, len(s.results))
	for _, r := range s.results {
		texts = append(texts, r.Text)
	}
	return "(" + params + ") (" + strings.Join(texts, ", ") + ")"
}

// arguments are the parameter names after the context, the values the fake
// records; a variadic is recorded as its one slice.
func (s signature) arguments() string {
	names := make([]string, 0, len(s.Params))
	for _, p := range s.Params {
		names = append(names, p.Name)
	}
	return strings.Join(names, ", ")
}

// packages is every package name the signature's types use.
func (s signature) packages() []string {
	out := s.paramPackages()
	if s.Context {
		out = append(out, "context")
	}
	for _, r := range s.results {
		out = append(out, r.Packages...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// paramPackages is every package name the parameters after the context use,
// the types a call function or a recording test declares values of.
func (s signature) paramPackages() []string {
	var out []string
	for _, p := range s.Params {
		out = append(out, p.Type.Packages...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// signatures shapes every method of a port in source order.
func signatures(p genkind.Port) ([]signature, error) {
	out := make([]signature, 0, len(p.Methods))
	for _, m := range p.Methods {
		s, err := shape(p, m)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}
