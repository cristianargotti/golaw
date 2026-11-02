package port

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
)

func TestQualifyPrefixesTheTypesOfTheContractPackage(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		expr, want string
		packages   []string
	}{
		"red: a bare type of the package":       {expr: "AccountID", want: "growth.AccountID", packages: []string{"growth"}},
		"red: a slice of the package's type":    {expr: "[]Plan", want: "[]growth.Plan", packages: []string{"growth"}},
		"red: a map and a pointer":              {expr: "map[AccountID]*Plan", want: "map[growth.AccountID]*growth.Plan", packages: []string{"growth"}},
		"red: a variadic":                       {expr: "...Option", want: "...growth.Option", packages: []string{"growth"}},
		"red: a generic instance":               {expr: "Page[Plan]", want: "growth.Page[growth.Plan]", packages: []string{"growth"}},
		"red: a func type keeps its names":      {expr: "func(plan Plan) error", want: "func(plan growth.Plan) error", packages: []string{"growth"}},
		"red: two packages in one type":         {expr: "map[AccountID]time.Time", want: "map[growth.AccountID]time.Time", packages: []string{"growth", "time"}},
		"near: a selector of another package":   {expr: "time.Time", want: "time.Time", packages: []string{"time"}},
		"near: a context":                       {expr: "context.Context", want: "context.Context", packages: []string{"context"}},
		"near: predeclared types":               {expr: "map[string][]byte", want: "map[string][]byte"},
		"near: error and any":                   {expr: "func(any) error", want: "func(any) error"},
		"near: a channel of a predeclared type": {expr: "<-chan int", want: "<-chan int"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := qualify(tc.expr, "growth")
			if err != nil {
				t.Fatal(err)
			}
			if got.Text != tc.want || !reflect.DeepEqual(got.Packages, tc.packages) {
				t.Errorf("qualify(%q) = %+v, want %q over %v", tc.expr, got, tc.want, tc.packages)
			}
		})
	}
}

func TestQualifyRefusesATypeItCannotRead(t *testing.T) {
	t.Parallel()
	if _, err := qualify("map[", "growth"); !errors.Is(err, genkind.ErrCannotJudge) {
		t.Errorf("qualify(map[) = %v, want ErrCannotJudge", err)
	}
}

func TestShapeReadsEachMethodAsTheFakeWritesIt(t *testing.T) {
	t.Parallel()
	ctx := genkind.Param{Name: "ctx", Type: "context.Context"}
	tests := map[string]struct {
		method       genkind.Method
		decl, args   string
		ctx, failing bool
	}{
		"red: a context and an error": {
			method: genkind.Method{Name: "ReadPlan", Params: []genkind.Param{ctx, {Name: "account", Type: "AccountID"}}, Results: []genkind.Param{{Type: "Plan"}, {Type: "error"}}},
			decl:   "(ctx context.Context, account growth.AccountID) (growth.Plan, error)", args: "account", ctx: true, failing: true,
		},
		"red: unnamed parameters get names": {
			method: genkind.Method{Name: "Read", Params: []genkind.Param{{Type: "context.Context"}, {Type: "AccountID"}}, Results: []genkind.Param{{Type: "error"}}},
			decl:   "(ctx context.Context, arg2 growth.AccountID) error", args: "arg2", ctx: true, failing: true,
		},
		"red: names the fake or an import already uses": {
			method: genkind.Method{Name: "Read", Params: []genkind.Param{{Name: "f", Type: "string"}, {Name: "time", Type: "time.Duration"}}},
			decl:   "(fArg string, timeArg time.Duration)", args: "fArg, timeArg",
		},
		"red: adjacent parameters of one type are grouped": {
			method: genkind.Method{Name: "Move", Params: []genkind.Param{ctx, {Name: "from", Type: "AccountID"}, {Name: "to", Type: "AccountID"}}, Results: []genkind.Param{{Type: "error"}}},
			decl:   "(ctx context.Context, from, to growth.AccountID) error", args: "from, to", ctx: true, failing: true,
		},
		"red: a context without an error result is left unused": {
			method: genkind.Method{Name: "Watch", Params: []genkind.Param{ctx}, Results: []genkind.Param{{Type: "<-chan Plan"}}},
			decl:   "(_ context.Context) <-chan growth.Plan", ctx: true,
		},
		"near: one type apart is not grouped": {
			method: genkind.Method{Name: "Mix", Params: []genkind.Param{{Name: "a", Type: "string"}, {Name: "n", Type: "int"}, {Name: "b", Type: "string"}}},
			decl:   "(a string, n int, b string)", args: "a, n, b",
		},
		"near: a variadic stays last and alone": {
			method: genkind.Method{Name: "Save", Params: []genkind.Param{ctx, {Name: "plans", Type: "...Plan"}}, Results: []genkind.Param{{Type: "error"}}},
			decl:   "(ctx context.Context, plans ...growth.Plan) error", args: "plans", ctx: true, failing: true,
		},
		"near: an error that is not last is a value": {
			method: genkind.Method{Name: "Last", Results: []genkind.Param{{Type: "error"}, {Type: "int"}}},
			decl:   "() (error, int)",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s, err := shape(ledgerPort(), tc.method)
			if err != nil {
				t.Fatal(err)
			}
			if got := s.declaration(); got != tc.decl {
				t.Errorf("declaration() = %q, want %q", got, tc.decl)
			}
			if got := s.arguments(); got != tc.args || s.Context != tc.ctx || s.Failing != tc.failing {
				t.Errorf("arguments() = %q, context %v, failing %v", got, s.Context, s.Failing)
			}
		})
	}
}

func TestShapeRefusesWhatTheFakeCannotCarry(t *testing.T) {
	t.Parallel()
	params := func(n int) []genkind.Param {
		out := make([]genkind.Param, n)
		for i := range out {
			out[i] = genkind.Param{Name: fmt.Sprintf("p%d", i), Type: "string"}
		}
		return out
	}
	tests := map[string]struct {
		method genkind.Method
		want   error
		text   string
	}{
		"red: eight parameters": {
			method: genkind.Method{Name: "Save", Line: 14, Params: params(8)},
			want:   genkind.ErrRequest, text: "port growth.Ledger method Save at internal/application/growth/ports.go:14 takes 8 parameters, over the 7 of argument-limit",
		},
		"red: a type that does not parse": {
			method: genkind.Method{Name: "Save", Line: 14, Params: []genkind.Param{{Name: "x", Type: "map["}}},
			want:   genkind.ErrCannotJudge, text: "does not parse",
		},
		"near: seven parameters": {method: genkind.Method{Name: "Save", Line: 14, Params: params(7)}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := shape(ledgerPort(), tc.method)
			if tc.want == nil {
				if err != nil {
					t.Errorf("shape = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), tc.text) {
				t.Errorf("shape = %v, want %v naming %q", err, tc.want, tc.text)
			}
		})
	}
}

func TestImportsForResolvesEveryPackageName(t *testing.T) {
	t.Parallel()
	p := genkind.Port{Package: "growth", Dir: "internal/application/growth", Imports: []genkind.Import{
		{Path: "context"}, {Path: "time"}, {Name: "yaml", Path: "gopkg.in/yaml.v3"},
	}}
	got, err := importsFor(p, "example.com/slice", []string{"time", "growth", "context", "yaml"}, "testing", "context")
	if err != nil {
		t.Fatal(err)
	}
	want := []genkind.Import{
		{Path: "context"},
		{Path: "example.com/slice/internal/application/growth"},
		{Name: "yaml", Path: "gopkg.in/yaml.v3"},
		{Path: "testing"},
		{Path: "time"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("importsFor = %+v, want %+v", got, want)
	}
}

func TestImportsForRefusesWhatItCannotResolve(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		imports []genkind.Import
		names   []string
		text    string
	}{
		"red: a package the contract does not import":    {imports: []genkind.Import{{Path: "time"}}, names: []string{"uuid"}, text: "names package uuid, which the contract file does not import"},
		"red: a dot import":                              {imports: []genkind.Import{{Name: ".", Path: "time"}}, names: []string{"time"}, text: "dot imports time"},
		"red: a path whose last element is not its name": {imports: []genkind.Import{{Path: "gopkg.in/yaml.v3"}}, names: []string{"yaml"}, text: "names package yaml"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := genkind.Port{Name: "Ledger", Package: "growth", Path: contractPath, Imports: tc.imports}
			if _, err := importsFor(p, "example.com/slice", tc.names); !errors.Is(err, genkind.ErrCannotJudge) || !strings.Contains(err.Error(), tc.text) {
				t.Errorf("importsFor = %v, want ErrCannotJudge naming %q", err, tc.text)
			}
		})
	}
}

func TestLowerFirstKeepsInitialismsWhole(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"ReadPlan": "readPlan", "HTTPGet": "httpGet", "ID": "id", "Now": "now", "A": "a"} {
		if got := lowerFirst(in); got != want {
			t.Errorf("lowerFirst(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPackagesIsTheUnionOfEveryTypeOfTheSignature(t *testing.T) {
	t.Parallel()
	s, err := shape(ledgerPort(), genkind.Method{
		Name:    "Wait",
		Params:  []genkind.Param{{Name: "ctx", Type: "context.Context"}, {Name: "d", Type: "time.Duration"}},
		Results: []genkind.Param{{Type: "Plan"}, {Type: "error"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := s.packages(), []string{"context", "growth", "time"}; !reflect.DeepEqual(got, want) {
		t.Errorf("packages() = %v, want %v", got, want)
	}
}

func TestShapeRenamesWhatWouldShadowTheFakeBody(t *testing.T) {
	t.Parallel()
	s, err := shape(ledgerPort(), genkind.Method{
		Name:    "Read",
		Params:  []genkind.Param{{Name: "f", Type: "context.Context"}, {Name: "porttest", Type: "string"}, {Name: "gensupport", Type: "int"}, {Name: "errors", Type: "int"}, {Name: "ledgertest", Type: "int"}},
		Results: []genkind.Param{{Type: "error"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := s.declaration(), "(fArg context.Context, porttestArg string, gensupportArg, errorsArg, ledgertestArg int) error"; got != want {
		t.Errorf("declaration() = %q, want %q", got, want)
	}
	if want := "porttest.Answer(fArg, f.rec, MethodRead, porttestArg, gensupportArg, errorsArg, ledgertestArg)"; !strings.Contains(fmt.Sprint(answer(s, "MethodRead")), want) {
		t.Errorf("answer = %q, want the call %q", fmt.Sprint(answer(s, "MethodRead")), want)
	}
}
