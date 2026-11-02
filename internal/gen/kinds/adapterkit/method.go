package adapterkit

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
)

const (
	contextType = "context.Context"
	errorType   = "error"
)

// Method is one port method in the shape the adapter templates write: every
// type qualified from the adapter's package and every name settled, so a
// template only joins strings and never decides.
type Method struct {
	Name    string
	Doc     []string        // doc comment lines, the first starting with Name
	Hole    string          // "plans.read-plan"
	Action  string          // "actionPlansReadPlan"
	Phrase  string          // "postgres: plans read plan"
	Params  []genkind.Param // qualified, every one named, the context first
	Results []genkind.Param // qualified, the error last
	Zeros   []genkind.Param // one variable per non-error result: zero, zero2
	Inputs  []genkind.Param // the non-context parameters as red test variables
	Line    int
}

// Signature spells the parameters and the results as a method declares them.
func (m Method) Signature() string {
	params := make([]string, 0, len(m.Params))
	for _, p := range m.Params {
		params = append(params, p.Name+" "+p.Type)
	}
	results := make([]string, 0, len(m.Results))
	for _, r := range m.Results {
		results = append(results, r.Type)
	}
	out := "(" + strings.Join(params, ", ") + ") "
	if len(results) == 1 {
		return out + results[0]
	}
	return out + "(" + strings.Join(results, ", ") + ")"
}

// Returns is what a hole returns before its error.
func (m Method) Returns() string { return joinEach(m.Zeros, "", ", ") }

// HoleArgs are the parameters a hole call passes after its id.
func (m Method) HoleArgs() string { return joinEach(m.Params, ", ", "") }

// Discard is one blank per non-error result in a red test's assignment.
func (m Method) Discard() string { return strings.Repeat("_, ", len(m.Zeros)) }

// CallArgs are the arguments a red test calls the method with.
func (m Method) CallArgs() string { return "t.Context()" + joinEach(m.Inputs, ", ", "") }

// Test is the name of the method's red test for adapter type typ.
func (m Method) Test(typ string) string { return "Test" + typ + m.Name }

// Message is the quoted text of an open hole's error.
func (m Method) Message(constraint string) string {
	return strconv.Quote("hole " + m.Hole + " is open: " + constraint)
}

func joinEach(ps []genkind.Param, before, after string) string {
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		parts = append(parts, before+p.Name+after)
	}
	return strings.Join(parts, "")
}

// Methods shapes every method of the target's port in source order and
// answers the imports their types need, sorted by path.
func Methods(t Target) ([]Method, []genkind.Import, error) {
	var out []Method
	var pkgs []string
	qualified := false
	for _, m := range t.Port.Methods {
		shaped, used, own, err := shape(t, m)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, shaped)
		pkgs = append(pkgs, used...)
		qualified = qualified || own
	}
	imports, err := importsOf(t, pkgs)
	if err != nil {
		return nil, nil, err
	}
	if qualified {
		imports = sortImports(append(imports, t.PortPkg))
	}
	return out, imports, nil
}

// shape refuses a method an adapter cannot implement safely and settles its
// names: an adapter call reaches I/O, so it must carry its caller's deadline
// and be able to fail.
func shape(t Target, m genkind.Method) (Method, []string, bool, error) {
	if len(m.Params) == 0 || m.Params[0].Type != contextType {
		return Method{}, nil, false, fmt.Errorf("%w: method %s of port %s has no context.Context first parameter: "+
			"an adapter call carries its caller's deadline; add it to the port", genkind.ErrRequest, m.Name, t.Port.Name)
	}
	if len(m.Results) == 0 || m.Results[len(m.Results)-1].Type != errorType {
		return Method{}, nil, false, fmt.Errorf("%w: method %s of port %s answers no error: an adapter call can fail; "+
			"add an error result or write the adapter by hand under golaw:handwritten", genkind.ErrRequest, m.Name, t.Port.Name)
	}
	params, pkgs, own, err := qualifyAll(m.Params, t.Port.Package, true)
	if err != nil {
		return Method{}, nil, false, fmt.Errorf("method %s of port %s: %w", m.Name, t.Port.Name, err)
	}
	results, resultPkgs, resultOwn, err := qualifyAll(m.Results, t.Port.Package, false)
	if err != nil {
		return Method{}, nil, false, fmt.Errorf("method %s of port %s: %w", m.Name, t.Port.Name, err)
	}
	hole := t.Hole(m.Name)
	zeros := zerosOf(results, params)
	return Method{
		Name: m.Name, Doc: docLines(m, t.Port.Name), Hole: hole,
		Action: "action" + t.Name.Type + m.Name,
		Phrase: t.Name.Area + ": " + t.Lower() + " " + strings.ReplaceAll(strings.TrimPrefix(hole, t.Lower()+"."), "-", " "),
		Params: params, Results: results, Zeros: zeros, Inputs: inputsOf(params, zeros), Line: m.Line,
	}, append(pkgs, resultPkgs...), own || resultOwn, nil
}

// qualifyAll qualifies every type of a list; named also gives an unnamed
// parameter a name, because a hole passes every parameter on.
func qualifyAll(list []genkind.Param, pkg string, named bool) ([]genkind.Param, []string, bool, error) {
	out := make([]genkind.Param, 0, len(list))
	var pkgs []string
	own := false
	for i, p := range list {
		typ, used, err := qualify(p.Type, pkg)
		if err != nil {
			return nil, nil, false, err
		}
		own = own || typ != p.Type
		pkgs = append(pkgs, used...)
		if named && (p.Name == "" || p.Name == "_") {
			p.Name = "arg" + strconv.Itoa(i)
			if i == 0 {
				p.Name = "ctx"
			}
		}
		out = append(out, genkind.Param{Name: p.Name, Type: typ})
	}
	return out, pkgs, own, nil
}
