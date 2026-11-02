package handler_test

import (
	"bytes"
	"errors"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/handler"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
)

const dir = "internal/adapters/in/plansapi/"

func fixtureOperations() []genkind.Operation {
	return []genkind.Operation{
		{Name: "GetPlan", Tag: "plans", Method: http.MethodGet, Route: "/plans/{id}", Line: 69},
		{Name: "ArchivePlan", Tag: "plans", Method: http.MethodPost, Route: "/plans/{id}/archive", Line: 72},
	}
}

func growthEntries() manifest.Manifest {
	m := manifest.Manifest{Schema: manifest.Schema, Support: manifest.Support{Path: "internal/gensupport", Template: "support@1"}}
	ports := "internal/application/growth/ports.go"
	entries := []manifest.Entry{
		{Kind: genkind.KindPort, Template: "port@1", Name: "growth.Clock", Contracts: []manifest.Contract{{Path: ports, Symbol: "Clock"}}},
		{Kind: genkind.KindPort, Template: "port@1", Name: "growth.Ledger", Contracts: []manifest.Contract{{Path: ports, Symbol: "Ledger"}}},
		{Kind: genkind.KindUsecase, Template: "usecase@1", Name: "growth.ArchivePlan", Ports: []string{"Ledger"}, In: "AccountID", Out: "Plan"},
		{Kind: genkind.KindUsecase, Template: "usecase@1", Name: "growth.ReadPlan", Ports: []string{"Ledger", "Clock"}, In: "AccountID", Out: "Plan"},
	}
	for i := range entries {
		m = m.With(entries[i])
	}
	return m
}

func newInput(t *testing.T, ops []genkind.Operation, files map[string][]byte, m manifest.Manifest) genkind.Input {
	t.Helper()
	return genkind.Input{
		Module: "example.com/slice", Version: "0.3.0", Policy: model.DefaultGenerate(),
		Snapshot: gentest.Snapshot(t, files), Manifest: m, Contracts: gentest.FakeContracts{OperationAnswers: ops},
	}
}

func newRequest() genkind.Request {
	return genkind.Request{Name: genkind.Name{Area: "plansapi"}, API: "api/openapi.yaml", Usecases: []string{"growth"}}
}

func filePaths(files []genkind.File) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

// written is the fixture tree with every file a run wrote.
func written(out genkind.Output) map[string][]byte {
	tree := gentest.Contracts()
	for _, f := range slices.Concat(out.Regenerable, out.Owned) {
		tree[f.Path] = f.Content
	}
	return tree
}

func TestKindIsTheHandlerOfTemplateOne(t *testing.T) {
	t.Parallel()
	if k := handler.Kind(); k.Name() != genkind.KindHandler || k.Template() != 1 {
		t.Fatalf("Kind() = %s@%d", k.Name(), k.Template())
	}
}

func TestNewWritesTheEntryTheFilesAndTheBinding(t *testing.T) {
	t.Parallel()
	out, err := handler.Kind().New(t.Context(), newInput(t, fixtureOperations(), gentest.Contracts(), growthEntries()), newRequest())
	if err != nil {
		t.Fatal(err)
	}
	want := manifest.Entry{
		Kind: genkind.KindHandler, Template: "handler@1", Name: "plansapi.Handler",
		Contracts: []manifest.Contract{{Path: "api/gen/server.gen.go"}, {Path: "api/openapi.yaml"}},
		Usecases:  []string{"growth"},
		Regenerable: []string{
			dir + "zz_generated_handler.go", dir + "zz_generated_handler_internal_test.go",
			dir + "zz_generated_handler_routes_test.go", dir + "zz_generated_handler_test.go",
		},
		Owned: []string{dir + "plans.go", dir + "plans_test.go", dir + "status.go", dir + "status_internal_test.go"},
	}
	if !reflect.DeepEqual(out.Entry, want) {
		t.Errorf("entry = %+v, want %+v", out.Entry, want)
	}
	if err := growthEntries().With(out.Entry).Validate(); err != nil {
		t.Errorf("the entry does not validate: %v", err)
	}
	if !slices.Equal(filePaths(out.Regenerable), want.Regenerable) || !slices.Equal(filePaths(out.Owned), want.Owned) {
		t.Errorf("files = %v and %v", filePaths(out.Regenerable), filePaths(out.Owned))
	}
	binding := []genkind.Binding{{
		Kind: genkind.KindHandler, Area: "growth", Import: genkind.Import{Path: "example.com/slice/internal/adapters/in/plansapi"},
		Call: "plansapi.NewHandler",
	}}
	if !reflect.DeepEqual(out.Bindings, binding) || len(out.Appends) != 0 || len(out.Decisions) != 0 || len(out.Needs) != 0 {
		t.Errorf("bindings %+v, appends %+v, decisions %+v, needs %+v", out.Bindings, out.Appends, out.Decisions, out.Needs)
	}
	for _, f := range slices.Concat(out.Regenerable, out.Owned) {
		if bytes.Contains(f.Content, []byte("goverter")) {
			t.Errorf("%s renders a goverter interface, which A54 keeps out of every handler template", f.Path)
		}
	}
}

func TestNewRefusesASecondHandlerOfTheName(t *testing.T) {
	t.Parallel()
	in := newInput(t, fixtureOperations(), gentest.Contracts(), growthEntries())
	out, err := handler.Kind().New(t.Context(), in, newRequest())
	if err != nil {
		t.Fatal(err)
	}
	in.Manifest = in.Manifest.With(out.Entry)
	_, err = handler.Kind().New(t.Context(), in, newRequest())
	if !errors.Is(err, genkind.ErrExists) || !strings.Contains(err.Error(), dir+"zz_generated_handler.go") {
		t.Fatalf("New over an existing handler = %v, want ErrExists naming its file", err)
	}
}

func TestRegenerateFollowsTheDocument(t *testing.T) {
	t.Parallel()
	first, err := handler.Kind().New(t.Context(), newInput(t, fixtureOperations()[:1], gentest.Contracts(), growthEntries()), newRequest())
	if err != nil {
		t.Fatal(err)
	}
	m := growthEntries().With(first.Entry)
	for name, tc := range map[string]struct {
		ops     []genkind.Operation
		appends []string
		route   bool
	}{
		"red: an operation added to the document":            {ops: fixtureOperations(), appends: []string{dir + "plans.go", dir + "plans_test.go"}, route: true},
		"near: the same document regenerates the same bytes": {ops: fixtureOperations()[:1]},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out, err := handler.Kind().Regenerate(t.Context(), newInput(t, tc.ops, written(first), m), first.Entry)
			if err != nil {
				t.Fatal(err)
			}
			checkRegenerated(t, first, out, tc.appends, tc.route)
		})
	}
}

// checkRegenerated holds the assertions of one regeneration apart from the
// table, so each row reads as its document and its expected answer.
func checkRegenerated(t *testing.T, first, out genkind.Output, appends []string, route bool) {
	t.Helper()
	appended := make([]string, 0, len(out.Appends))
	for _, a := range out.Appends {
		appended = append(appended, a.Path)
	}
	if !slices.Equal(appended, appends) || len(out.Owned) != 0 || len(out.Decisions) != 0 ||
		!reflect.DeepEqual(out.Entry, first.Entry) {
		t.Errorf("appends %v, owned %v, decisions %+v, entry %+v", appended, filePaths(out.Owned), out.Decisions, out.Entry)
	}
	for i, f := range out.Regenerable {
		same := bytes.Equal(f.Content, first.Regenerable[i].Content)
		if strings.HasSuffix(f.Path, "_routes_test.go") && same == route {
			t.Errorf("%s unchanged %v, want a new route %v", f.Path, same, route)
		}
	}
}

func TestRegenerateRefusesAnEntryWithoutItsContracts(t *testing.T) {
	t.Parallel()
	in := newInput(t, fixtureOperations(), gentest.Contracts(), growthEntries())
	out, err := handler.Kind().New(t.Context(), in, newRequest())
	if err != nil {
		t.Fatal(err)
	}
	broken := out.Entry
	broken.Contracts = nil
	if _, err := handler.Kind().Regenerate(t.Context(), in, broken); !errors.Is(err, manifest.ErrManifest) {
		t.Fatalf("Regenerate = %v, want ErrManifest", err)
	}
}
