package port

import (
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
)

// answer writes the body of a fake method: the call that records it and
// the return of what Program set. The values are read left to right in one
// return statement, which Go evaluates in lexical order, so no index
// literal is written and mnd stays quiet at any arity.
func answer(s signature, constant string) (string, string) {
	args := constant
	if list := s.arguments(); list != "" {
		args += ", " + list
	}
	values := nexts(s.Values, "r")
	if !s.Failing {
		call := "porttest.Record(f.rec, " + args + ")"
		if values == "" {
			return call, ""
		}
		return "r := " + call, "return " + values
	}
	call := "porttest.Failing(f.rec, " + args + ")"
	if s.Context {
		call = "porttest.Answer(" + s.ctxName + ", f.rec, " + args + ")"
	}
	if values == "" {
		return "_, err := " + call, "return err"
	}
	return "r, err := " + call, "return " + values + ", err"
}

// nexts reads each value of an answer as its type.
func nexts(types []typeExpr, from string) string {
	out := make([]string, 0, len(types))
	for _, t := range types {
		out = append(out, "porttest.Next["+t.Text+"]("+from+")")
	}
	return strings.Join(out, ", ")
}

// callSignature is the suite's call function of one method; a context or
// arguments the method does not take stay blank, so unparam keeps quiet.
func callSignature(p genkind.Port, s signature) string {
	ctx, args := blank, blank
	if s.Context {
		ctx = ctxParam
	}
	if len(s.Params) > 0 {
		args = "args"
	}
	return "(" + ctx + " context.Context, p " + p.Package + "." + p.Name + ", " + args + " []any) []any"
}

// invoke is the suite's call of one method with its arguments read from the
// case; a method without results cannot be packed, so it answers nil.
func invoke(s signature) string {
	var args []string
	if s.Context {
		args = append(args, ctxParam)
	}
	for _, p := range s.Params {
		arg := "porttest.Next[" + sliceOf(p.Type.Text) + "](in)"
		if variadicType(p.Type.Text) {
			arg += variadic
		}
		args = append(args, arg)
	}
	call := "p." + s.Name + "(" + strings.Join(args, ", ") + ")"
	if len(s.results) == 0 {
		return call + "\nreturn nil"
	}
	return "return porttest.Pack(" + call + ")"
}

// recordingCall is the recording test's call of the first method over zero
// valued variables, keeping the error when the method answers one.
func recordingCall(s signature) string {
	var args []string
	if s.Context {
		args = append(args, "t.Context()")
	}
	for _, p := range s.Params {
		arg := p.Name
		if variadicType(p.Type.Text) {
			arg += variadic
		}
		args = append(args, arg)
	}
	call := "fake." + s.Name + "(" + strings.Join(args, ", ") + ")"
	if !s.Failing {
		return call
	}
	return strings.Repeat("_, ", len(s.Values)) + "err := " + call
}

// sliceOf spells a variadic parameter as the slice it is inside the body.
func sliceOf(text string) string {
	if rest, ok := strings.CutPrefix(text, variadic); ok {
		return "[]" + rest
	}
	return text
}

func variadicType(text string) bool {
	return strings.HasPrefix(text, variadic)
}
