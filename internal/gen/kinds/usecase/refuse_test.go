package usecase_test

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/usecase"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
)

// fill answers n use case entries of growth, sorted, each over Ledger.
func fill(n int) []manifest.Entry {
	out := make([]manifest.Entry, 0, 1+n)
	out = append(out, portEntry("Ledger"))
	for i := range n {
		out = append(out, manifest.Entry{Kind: genkind.KindUsecase, Template: "usecase@1", Name: fmt.Sprintf("growth.Use%02d", i), Ports: []string{"Ledger"}, In: "AccountID", Out: "Plan"})
	}
	return out
}

func TestNewUsecaseRefusesWhatItCannotGenerate(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		entries []manifest.Entry
		port    string
		extra   map[string]string
		req     genkind.Request
		want    error
		text    string
	}{
		"red: no port":                                         {req: genkind.Request{Name: asked, In: "AccountID", Out: "Plan"}, want: genkind.ErrRequest, text: "golaw new usecase needs --ports"},
		"red: a port twice":                                    {req: readPlan("Ledger", "Ledger"), want: genkind.ErrRequest, text: "names port Ledger twice"},
		"red: a port that is not exported":                     {req: readPlan("ledger"), want: genkind.ErrRequest, text: "ledger is not an exported Go name"},
		"red: an input of another package":                     {req: genkind.Request{Name: asked, Ports: []string{"Ledger"}, In: "time.Time"}, want: genkind.ErrRequest, text: "time.Time is not an exported Go name"},
		"red: an area that is not a package name":              {req: genkind.Request{Name: genkind.Name{Area: "Growth", Type: "ReadPlan"}, Ports: []string{"Ledger"}}, want: genkind.ErrRequest, text: "is not <area>.<Type>"},
		"red: the same use case again":                         {entries: []manifest.Entry{portEntry("Ledger"), entry("ReadPlan", "read_plan", "Ledger")}, req: readPlan("Ledger"), want: genkind.ErrExists, text: area + "read_plan.go"},
		"red: another input over the entry":                    {entries: []manifest.Entry{entry("ReadPlan", "read_plan", "Ledger")}, req: genkind.Request{Name: asked, Ports: []string{"Ledger"}, In: "Plan", Out: "Plan"}, want: genkind.ErrRequest, text: "reads AccountID and answers Plan, fixed by internal/application/growth/read_plan.go"},
		"red: ports outside the consuming package":             {port: "internal/domain/{area}", req: readPlan("Ledger"), want: genkind.ErrRequest, text: "the same directory"},
		"red: one use case over the area limit":                {entries: fill(36), req: readPlan("Ledger"), want: genkind.ErrRequest, text: "over the 36"},
		"near: the entry with its ports changed":               {entries: []manifest.Entry{entry("ReadPlan", "read_plan", "Ledger")}, req: genkind.Request{Name: asked, Ports: []string{"Ledger", "Clock"}}},
		"near: the last use case under the limit":              {entries: fill(35), req: readPlan("Ledger")},
		"red: an area with hand written Deps":                  {extra: map[string]string{area + "use_cases.go": "package growth\n\n// Deps are the ports of growth, wired by hand.\ntype Deps struct{}\n"}, req: readPlan("Ledger"), want: genkind.ErrRequest, text: "declares Deps by hand in internal/application/growth/use_cases.go"},
		"red: an area with hand written NewUseCases":           {extra: map[string]string{area + "use_cases.go": "package growth\n\n// NewUseCases wires growth by hand.\nfunc NewUseCases() {}\n"}, req: readPlan("Ledger"), want: genkind.ErrRequest, text: "declares NewUseCases by hand"},
		"near: a hand written name that only starts like Deps": {extra: map[string]string{area + "deps_of.go": "package growth\n\n// DepsOf names the dependencies of a plan.\ntype DepsOf struct{}\n"}, req: readPlan("Ledger")},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			files := gentest.Contracts()
			for path, content := range tc.extra {
				files[path] = []byte(content)
			}
			in := input(t, files, tc.entries...)
			if tc.port != "" {
				in.Policy.Layout.Port = tc.port
			}
			_, err := usecase.Kind().New(t.Context(), in, tc.req)
			assertRefusal(t, err, tc.want, tc.text)
		})
	}
}

// assertRefusal holds a near row to no error and a red row to its sentinel
// naming its text.
func assertRefusal(t *testing.T, err, want error, text string) {
	t.Helper()
	if want == nil {
		if err != nil {
			t.Errorf("New = %v, want nil", err)
		}
		return
	}
	if !errors.Is(err, want) || !strings.Contains(err.Error(), text) {
		t.Errorf("New = %v, want %v naming %q", err, want, text)
	}
}

func TestAnAreaAtTheLimitStaysWithinTheSizeLaws(t *testing.T) {
	t.Parallel()
	out, err := usecase.Kind().New(t.Context(), input(t, gentest.Contracts(), fill(35)...), readPlan("Ledger"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range append(out.Regenerable, out.Owned...) {
		if lines := strings.Count(string(f.Content), "\n"); lines > 300 {
			t.Errorf("%s holds %d lines, over 300", f.Path, lines)
		}
		if strings.HasSuffix(f.Path, "_test.go") {
			continue
		}
		assertFunctionsWithin40(t, f.Path, f.Content)
	}
}

// assertFunctionsWithin40 holds every function body of one file to the 40
// line law.
func assertFunctionsWithin40(t *testing.T, path string, content []byte) {
	t.Helper()
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, path, content, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
			if n := fset.Position(fn.Body.Rbrace).Line - fset.Position(fn.Body.Lbrace).Line - 1; n > 40 {
				t.Errorf("%s %s spans %d lines, over 40", path, fn.Name.Name, n)
			}
		}
	}
}
