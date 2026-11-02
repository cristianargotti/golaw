package render_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/render"
)

func TestParamsNamesAndResultsSpellAMethod(t *testing.T) {
	t.Parallel()
	ps := []genkind.Param{{Name: "ctx", Type: "context.Context"}, {Type: "AccountID"}, {Name: "_", Type: "string"}}
	if got := render.Params(ps); got != "ctx context.Context, _ AccountID, _ string" {
		t.Errorf("Params = %q", got)
	}
	if got := render.Names(ps); got != "ctx" {
		t.Errorf("Names = %q", got)
	}
	for want, rs := range map[string][]genkind.Param{
		"":              nil,
		"error":         {{Type: "error"}},
		"(Plan, error)": {{Name: "p", Type: "Plan"}, {Type: "error"}},
	} {
		if got := render.Results(rs); got != want {
			t.Errorf("Results(%v) = %q, want %q", rs, got, want)
		}
	}
	if got := render.LowerFirst("ReadPlan"); got != "readPlan" {
		t.Errorf("LowerFirst = %q", got)
	}
}

func TestZeroAnswersOnlyWhatTheSyntaxProves(t *testing.T) {
	t.Parallel()
	for typ, want := range map[string]string{
		"bool": "false", "string": `""`, "int64": "0", "float64": "0", "byte": "0", "rune": "0",
		"error": "nil", "any": "nil", "*Plan": "nil", "[]Plan": "nil", "map[string]int": "nil",
		"chan int": "nil", "func()": "nil", "interface{}": "nil",
		"[2]int": "[2]int{}", "struct{}": "struct{}{}",
		"Plan": "", "growth.Plan": "", "time.Time": "", "AccountID": "", "[": "",
	} {
		if got := render.Zero(typ); got != want {
			t.Errorf("Zero(%q) = %q, want %q", typ, got, want)
		}
	}
}

func TestReturnZeroDeclaresWhatItCannotSpell(t *testing.T) {
	t.Parallel()
	e := genkind.Param{Type: "error"}
	tests := map[string]struct {
		results, params []genkind.Param
		want            string
	}{
		"a named result":         {[]genkind.Param{{Type: "Plan"}, e}, []genkind.Param{{Name: "ctx"}}, "var zero Plan\nreturn zero, E"},
		"an error alone":         {[]genkind.Param{e}, nil, "return E"},
		"builtins":               {[]genkind.Param{{Type: "string"}, {Type: "int"}, e}, nil, "return \"\", 0, E"},
		"a parameter named zero": {[]genkind.Param{{Type: "A"}, {Type: "B"}, e}, []genkind.Param{{Name: "zero"}}, "var zero2 A\nvar zero3 B\nreturn zero2, zero3, E"},
		"no result":              {nil, nil, "return"},
		"no error result":        {[]genkind.Param{{Type: "time.Time"}}, nil, "var zero time.Time\nreturn zero"},
		"a pointer":              {[]genkind.Param{{Type: "*Plan"}, e}, nil, "return nil, E"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := render.ReturnZero(tc.results, tc.params, "E"); got != tc.want {
				t.Errorf("ReturnZero = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestQualifyPrefixesTheContractPackage(t *testing.T) {
	t.Parallel()
	for typ, want := range map[string]string{
		"AccountID": "growth.AccountID", "[]Plan": "[]growth.Plan", "map[string]*Plan": "map[string]*growth.Plan",
		"context.Context": "context.Context", "error": "error", "func(Plan) error": "func(growth.Plan) error",
		"Page[Plan]": "growth.Page[growth.Plan]", "struct{ Name Plan }": "struct{ Name growth.Plan }",
	} {
		if got, err := render.Qualify("growth", typ); err != nil || got != want {
			t.Errorf("Qualify(%q) = %q, %v, want %q", typ, got, err, want)
		}
	}
	if _, err := render.Qualify("growth", "["); err == nil {
		t.Error("a type that does not parse answered no error")
	}
	if got, err := render.Qualify("", "Plan"); err != nil || got != "Plan" {
		t.Errorf("Qualify without a package = %q, %v", got, err)
	}
	if len(render.Funcs()) != 8 {
		t.Errorf("Funcs = %d functions, want kebab, lowerFirst, qualify, params, names, results, zero, returnZero", len(render.Funcs()))
	}
}
