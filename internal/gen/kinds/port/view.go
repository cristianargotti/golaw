package port

import (
	"path"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
)

// view is what the port templates render from: every name and statement is
// decided here, so a template only places strings and stays free of logic
// a law could not see.
type view struct {
	Marker, Package, Area, Port, Hole, Constraint string
	Doc, Concat                                   bool
	Methods                                       []method
	First                                         *first
}

// method is one port method as the fake and the suite write it.
type method struct {
	Name, Const, Hole, Cases, Call, Declaration, Answer, Return, CallSignature, Invoke string
	Context, Failing, Values                                                           bool
}

// first is the method the recording test calls: one call is enough to
// reach Program and Calls, which no behavioral case reaches while the
// cases are still holes.
type first struct {
	Name, Const, Invoke string
	Vars                []string
	Failing             bool
}

// newView shapes every method of the port and fills the view.
func newView(p genkind.Port) (view, error) {
	sigs, err := signatures(p)
	if err != nil {
		return view{}, err
	}
	lower := strings.ToLower(p.Name)
	v := view{
		Marker:  holes.Marker,
		Package: path.Base(testDir(p)),
		Area:    p.Package,
		Port:    p.Name,
		Hole:    holes.ID(lower, "Methods"),
		Constraint: "declare the methods of " + p.Name + " in " + p.Path +
			": each takes a context.Context first and answers an error last, at most " + strconv.Itoa(maxParams) + " parameters",
		Concat: len(sigs) > 1,
	}
	for _, s := range sigs {
		v.Methods = append(v.Methods, newMethod(p, lower, s))
	}
	if len(sigs) > 0 {
		v.First = newFirst(sigs[0])
	}
	return v, nil
}

// newMethod names one method's constant, hole and functions and writes its
// statements.
func newMethod(p genkind.Port, lower string, s signature) method {
	m := method{
		Name:          s.Name,
		Const:         "Method" + s.Name,
		Hole:          holes.ID(lower, s.Name),
		Cases:         lowerFirst(s.Name) + "Cases",
		Call:          "call" + s.Name,
		Declaration:   s.declaration(),
		CallSignature: callSignature(p, s),
		Invoke:        invoke(s),
		Context:       s.Context,
		Failing:       s.Failing,
		Values:        len(s.Params) > 0,
	}
	m.Answer, m.Return = answer(s, m.Const)
	return m
}

// newFirst writes the recording test's call of one method.
func newFirst(s signature) *first {
	f := &first{Name: s.Name, Const: "Method" + s.Name, Invoke: recordingCall(s), Failing: s.Failing}
	for _, p := range s.Params {
		f.Vars = append(f.Vars, "var "+p.Name+" "+sliceOf(p.Type.Text))
	}
	return f
}
