package handler

import (
	"net/http"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
)

const (
	module     = "example.com/slice"
	handlerDir = "internal/adapters/in/plansapi/"
	growthPort = "internal/application/growth/ports.go"
)

// operations are the two operations of the growth fixture document, in the
// order the strict server interface declares them.
func operations() []genkind.Operation {
	return []genkind.Operation{
		{Name: "GetPlan", Tag: "plans", Method: http.MethodGet, Route: "/plans/{id}", Line: 69},
		{Name: "ArchivePlan", Tag: "plans", Method: http.MethodPost, Route: "/plans/{id}/archive", Line: 72},
	}
}

func growthManifest() manifest.Manifest {
	m := manifest.Manifest{Schema: manifest.Schema, Support: manifest.Support{Path: "internal/gensupport", Template: "support@1"}}
	entries := []manifest.Entry{
		{Kind: genkind.KindPort, Template: "port@1", Name: "growth.Clock", Contracts: []manifest.Contract{{Path: growthPort, Symbol: "Clock"}}},
		{Kind: genkind.KindPort, Template: "port@1", Name: "growth.Ledger", Contracts: []manifest.Contract{{Path: growthPort, Symbol: "Ledger"}}},
		{Kind: genkind.KindUsecase, Template: "usecase@1", Name: "growth.ArchivePlan", Ports: []string{"Ledger"}, In: "AccountID", Out: "Plan"},
		{Kind: genkind.KindUsecase, Template: "usecase@1", Name: "growth.ReadPlan", Ports: []string{"Ledger", "Clock"}, In: "AccountID", Out: "Plan"},
	}
	for i := range entries {
		m = m.With(entries[i])
	}
	return m
}

// withBilling adds a second area whose use case consumes a port named like
// one of growth's, so the two fake packages share a name.
func withBilling(m manifest.Manifest) manifest.Manifest {
	ports := "internal/application/billing/ports.go"
	m = m.With(manifest.Entry{Kind: genkind.KindPort, Template: "port@1", Name: "billing.Ledger", Contracts: []manifest.Contract{{Path: ports, Symbol: "Ledger"}}})
	return m.With(manifest.Entry{Kind: genkind.KindUsecase, Template: "usecase@1", Name: "billing.ChargePlan", Ports: []string{"Ledger"}, In: "AccountID", Out: "Plan"})
}

func without(m manifest.Manifest, kind, name string) manifest.Manifest {
	out := manifest.Manifest{Schema: m.Schema, Support: m.Support}
	for i := range m.Entries {
		if e := m.Entries[i]; e.Kind != kind || e.Name != name {
			out = out.With(e)
		}
	}
	return out
}

func handlerEntry(pkg string) manifest.Entry {
	return manifest.Entry{
		Kind: genkind.KindHandler, Template: "handler@1", Name: pkg + ".Handler",
		Contracts:   []manifest.Contract{{Path: "api/gen/server.gen.go"}, {Path: "api/openapi.yaml"}},
		Usecases:    []string{"growth"},
		Regenerable: []string{"internal/adapters/in/" + pkg + "/zz_generated_handler.go"},
	}
}

func input(t *testing.T) genkind.Input {
	t.Helper()
	return genkind.Input{
		Module: module, Version: "0.3.0", Policy: model.DefaultGenerate(),
		Snapshot: gentest.Snapshot(t, gentest.Contracts()), Manifest: growthManifest(),
		Contracts: gentest.FakeContracts{OperationAnswers: operations()},
	}
}

func request() genkind.Request {
	return genkind.Request{Name: genkind.Name{Area: "plansapi"}, API: "api/openapi.yaml", Usecases: []string{"growth"}}
}

func viewOf(t *testing.T, ops []genkind.Operation) view {
	t.Helper()
	in := input(t)
	in.Contracts = gentest.FakeContracts{OperationAnswers: ops}
	v, err := newView(in, request())
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func filesWithout(paths ...string) map[string][]byte {
	files := gentest.Contracts()
	for _, path := range paths {
		delete(files, path)
	}
	return files
}
