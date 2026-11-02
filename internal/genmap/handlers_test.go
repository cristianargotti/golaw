package genmap_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/genmap"
	"github.com/cristianargotti/golaw/internal/model"
)

const (
	plansAPI   = "internal/adapters/in/plansapi/handler.go"
	handlerSrc = "package plansapi\n\nimport \"example.com/slice/api/gen\"\n\n// Handler serves the plans.\ntype Handler struct{}\n\nvar _ apigen.StrictServerInterface = &Handler{}\n\nvar _ apigen.Other = &Handler{}\n"
	broken     = "package postgres\n\nfunc ( {\n"
	billing    = "internal/adapters/out/billing/client.go"
	billingSrc = "// golaw:handwritten\npackage billing\n\nimport \"example.com/slice/internal/application/growth\"\n\n// Client calls the billing API.\ntype Client struct{}\n\nvar (\n\t_ growth.Ledger = (*Client)(nil)\n\t_ growth.Clock  = (*Client)(nil)\n\t_ = growth.Plan{}\n)\n"
)

func handlerManifest() *manifest.Manifest {
	m := growthManifest()
	m.Entries = append(m.Entries,
		manifest.Entry{Kind: genkind.KindHandler, Template: "handler@1", Name: "plansapi.Plans", Usecases: []string{"growth.ReadPlan"}, Owned: []string{"internal/adapters/in/plansapi/plans.go"}},
		manifest.Entry{Kind: genkind.KindAdapterHTTP, Template: "adapter-http@1", Name: "billing.Remote", Contracts: []manifest.Contract{{Path: "api/billing.yaml"}}},
		manifest.Entry{Kind: genkind.KindPort, Template: "port@1", Name: "growth.Gone", Contracts: []manifest.Contract{{Path: "internal/application/growth/gone.go", Symbol: "Gone"}}},
	)
	return m
}

func TestBuildReadsTheHandlersAndTheAssertionsOfEveryAdapter(t *testing.T) {
	t.Parallel()
	extra := map[string]string{plansAPI: handlerSrc, "internal/adapters/out/postgres/broken.go": broken, billing: billingSrc, plansPath: ownedPlans}
	r, err := genmap.Build(t.Context(), genmap.Input{Snapshot: growthSnapshot(t, extra), Manifest: handlerManifest(), Policy: model.DefaultGenerate()})
	if err != nil {
		t.Fatal(err)
	}
	if got := names(r.Handlers); !slices.Equal(got, []string{"plansapi.Handler", "plansapi.Plans"}) {
		t.Fatalf("handlers = %+v", r.Handlers)
	}
	if !slices.Equal(r.Handlers[0].Details, []string{"implements apigen.StrictServerInterface"}) || !slices.Equal(r.Handlers[1].Details, []string{"usecases growth.ReadPlan"}) {
		t.Errorf("handlers = %+v", r.Handlers)
	}
	i := slices.IndexFunc(r.Adapters, func(it genmap.Item) bool { return it.Name == "billing.Client" })
	if i < 0 || !slices.Equal(r.Adapters[i].Details, []string{"port growth.Ledger", "port growth.Clock"}) || r.Adapters[i].Note != "handwritten without a reason" {
		t.Errorf("adapters = %+v", r.Adapters)
	}
	if j := slices.IndexFunc(r.Ports, func(it genmap.Item) bool { return it.Name == "growth.Gone" }); j < 0 || r.Ports[j].Details != nil {
		t.Errorf("ports = %+v", r.Ports)
	}
	area, err := genmap.Build(t.Context(), genmap.Input{Snapshot: growthSnapshot(t, extra), Manifest: handlerManifest(), Policy: model.DefaultGenerate(), Area: "growth"})
	if err != nil || !slices.Equal(names(area.Handlers), []string{"plansapi.Plans"}) || !slices.Contains(area.Contracts, portsPath) {
		t.Errorf("growth = %+v, %v", area, err)
	}
	bare, err := genmap.Build(t.Context(), genmap.Input{Snapshot: growthSnapshot(t, extra), Policy: model.DefaultGenerate(), Area: "growth"})
	if err != nil || len(bare.Holes) != 0 {
		t.Errorf("an area without a manifest keeps the hole of an adapter: %+v, %v", bare.Holes, err)
	}
}

func TestBuildStopsOnACanceledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := genmap.Build(ctx, genmap.Input{Snapshot: growthSnapshot(t, nil), Policy: model.DefaultGenerate()}); err == nil {
		t.Fatal("a canceled build answered no error")
	}
}

func TestTextNamesTheCommandOfEachOmission(t *testing.T) {
	t.Parallel()
	r := genmap.Report{Layers: []string{"port internal/application/{area}"}}
	for range 40 {
		r.Adapters = append(r.Adapters, genmap.Item{Name: "plain", Path: "internal/adapters/out/plain/plain.go", Note: "handwritten: streams"})
		r.Contracts = append(r.Contracts, "db/queries/plans.sql")
	}
	text := r.Text(100)
	for _, want := range []string{" more adapters: golaw map --json\n", " more contracts: golaw map --json\n"} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
	if whole := r.Text(1 << 20); !strings.Contains(whole, "adapter plain internal/adapters/out/plain/plain.go (handwritten: streams)\n") || !strings.Contains(whole, "contract db/queries/plans.sql\n") {
		t.Errorf("whole =\n%s", whole)
	}
}

func TestJSONWritesEmptySectionsAsArrays(t *testing.T) {
	t.Parallel()
	data, err := genmap.Report{}.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	for key, raw := range doc {
		if string(raw) != "[]" {
			t.Errorf("%s = %s, want []", key, raw)
		}
	}
}
