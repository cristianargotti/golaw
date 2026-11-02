package port

import (
	"fmt"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/model"
)

const contractPath = "internal/application/growth/ports.go"

// ledgerPort is the Ledger of the growth fixture (gentest.Contracts) as
// the contract reader answers it.
func ledgerPort() genkind.Port {
	ctx := genkind.Param{Name: "ctx", Type: "context.Context"}
	account := genkind.Param{Name: "account", Type: "AccountID"}
	return genkind.Port{
		Name: "Ledger", Package: "growth", Path: contractPath, Dir: "internal/application/growth", Line: 10,
		Imports: []genkind.Import{{Path: "context"}, {Path: "time"}},
		Methods: []genkind.Method{
			{Name: "ReadPlan", Line: 12, Params: []genkind.Param{ctx, account}, Results: []genkind.Param{{Type: "Plan"}, {Type: "error"}}},
			{Name: "ArchivePlan", Line: 14, Params: []genkind.Param{ctx, account}, Results: []genkind.Param{{Type: "error"}}},
		},
	}
}

const module = "example.com/slice"

func input() genkind.Input {
	return genkind.Input{Module: module, Version: "0.3.0", Policy: model.DefaultGenerate()}
}

func clockPort() genkind.Port {
	p := ledgerPort()
	p.Name, p.Line = "Clock", 18
	p.Methods = []genkind.Method{{Name: "Now", Line: 20, Results: []genkind.Param{{Type: "time.Time"}}}}
	return p
}

// shapesPort carries one method of every body shape the fake writes.
func shapesPort() genkind.Port {
	p := ledgerPort()
	ctx := genkind.Param{Name: "ctx", Type: "context.Context"}
	account := genkind.Param{Name: "account", Type: "AccountID"}
	p.Methods = []genkind.Method{
		{Name: "Forget", Line: 12, Params: []genkind.Param{ctx, account}},
		{Name: "Validate", Line: 13, Params: []genkind.Param{{Name: "plan", Type: "Plan"}}, Results: []genkind.Param{{Type: "error"}}},
		{Name: "Save", Line: 14, Params: []genkind.Param{ctx, {Name: "plans", Type: "...Plan"}}, Results: []genkind.Param{{Type: "error"}}},
	}
	return p
}

// stubPort is an interface the agent has not given methods yet.
func stubPort() genkind.Port {
	p := ledgerPort()
	p.Methods, p.Imports = nil, nil
	return p
}

// limitPort sits at every limit a port can reach: ten methods
// (interfacebloat) of seven parameters (argument-limit).
func limitPort() genkind.Port {
	p := ledgerPort()
	p.Methods = nil
	for i := range 10 {
		params := append(make([]genkind.Param, 0, maxParams), genkind.Param{Name: "ctx", Type: "context.Context"})
		for _, name := range []string{"a", "b", "c", "d", "e", "g"} {
			params = append(params, genkind.Param{Name: name, Type: "AccountID"})
		}
		p.Methods = append(p.Methods, genkind.Method{Name: fmt.Sprintf("Op%d", i), Line: 12 + i, Params: params, Results: []genkind.Param{{Type: "Plan"}, {Type: "error"}}})
	}
	return p
}

func rendered(t *testing.T, p genkind.Port) map[string]string {
	t.Helper()
	v, err := newView(p)
	if err != nil {
		t.Fatal(err)
	}
	files, err := regenerable(input(), p, v)
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]string, len(files))
	for _, f := range files {
		out[f.Path] = string(f.Content)
	}
	return out
}

// assertHas checks each want against content with every run of blanks
// read as one space, so gofmt's alignment never decides a test.
func assertHas(t *testing.T, content string, wants ...string) {
	t.Helper()
	normal := strings.Join(strings.Fields(content), " ")
	for _, want := range wants {
		if !strings.Contains(normal, strings.Join(strings.Fields(want), " ")) {
			t.Errorf("content lacks %q:\n%s", want, content)
		}
	}
}

func importPaths(t *testing.T, content string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", content, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(f.Imports))
	for _, spec := range f.Imports {
		out = append(out, strings.Trim(spec.Path.Value, `"`))
	}
	return out
}
