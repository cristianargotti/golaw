package genmap_test

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/genmap"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
)

const (
	portsPath  = "internal/application/growth/ports.go"
	plansPath  = "internal/adapters/out/postgres/plans.go"
	streamPath = "internal/adapters/out/vendor/stream.go"
	ownedPlans = "package postgres\n\nimport (\n\t\"context\"\n\n\t\"example.com/slice/internal/application/growth\"\n\t\"example.com/slice/internal/gensupport\"\n)\n\n// ReadPlan reads the plan of one account.\nfunc (p *Plans) ReadPlan(ctx context.Context, account growth.AccountID) (growth.Plan, error) {\n\t// golaw:hole plans.read-plan call p.queries.ReadPlan; map sql.ErrNoRows to growth.ErrPlanNotFound\n\treturn growth.Plan{}, gensupport.Hole(\"plans.read-plan\", ctx, account)\n}\n"
	stream     = "// golaw:handwritten the vendor API streams a chunked body\npackage vendor\n\nimport \"example.com/slice/internal/application/growth\"\n\n// Stream reads plans from the vendor.\ntype Stream struct{}\n\nvar _ growth.Ledger = (*Stream)(nil)\n"
	readPlan   = "package growth\n\nimport \"context\"\n\n// ReadPlan reads the plan of an account.\ntype ReadPlan struct{}\n\n// Execute reads the plan.\nfunc (u *ReadPlan) Execute(ctx context.Context, account AccountID) (Plan, error) {\n\treturn Plan{}, nil\n}\n"
)

func growthManifest() *manifest.Manifest {
	return &manifest.Manifest{Schema: manifest.Schema, Support: manifest.Support{Path: "internal/gensupport", Template: "support@1"}, Entries: []manifest.Entry{
		{
			Kind: genkind.KindAdapterSQL, Template: "adapter-sql@1", Name: "postgres.Plans",
			Contracts:   []manifest.Contract{{Path: portsPath, Symbol: "Ledger"}, {Path: "db/queries/plans.sql"}},
			Regenerable: []string{"internal/adapters/out/postgres/zz_generated_plans.go"}, Owned: []string{plansPath},
		},
		{
			Kind: genkind.KindPort, Template: "port@1", Name: "growth.Ledger",
			Contracts: []manifest.Contract{{Path: portsPath, Symbol: "Ledger"}},
		},
		{
			Kind: genkind.KindUsecase, Template: "usecase@1", Name: "growth.ReadPlan", Ports: []string{"Ledger", "Clock"}, In: "ReadPlanInput", Out: "Plan",
			Owned: []string{"internal/application/growth/read_plan.go"},
		},
	}}
}

func growthSnapshot(t *testing.T, extra map[string]string) model.Snapshot {
	t.Helper()
	files := gentest.Contracts()
	for p, text := range extra {
		files[p] = []byte(text)
	}
	return gentest.Snapshot(t, files)
}

func names(items []genmap.Item) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Name)
	}
	return out
}

func TestBuildMapsTheRepositoryFromTheManifestAndTheSources(t *testing.T) {
	t.Parallel()
	in := genmap.Input{Snapshot: growthSnapshot(t, map[string]string{plansPath: ownedPlans, streamPath: stream}), Manifest: growthManifest(), Policy: model.DefaultGenerate()}
	r, err := genmap.Build(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	if got := names(r.Ports); !slices.Equal(got, []string{"growth.Clock", "growth.Ledger"}) {
		t.Errorf("ports = %v", got)
	}
	if r.Ports[1].Path != portsPath || !slices.Equal(r.Ports[1].Details, []string{"methods ReadPlan, ArchivePlan"}) {
		t.Errorf("Ledger = %+v", r.Ports[1])
	}
	if len(r.Holes) != 1 || r.Holes[0].ID != "plans.read-plan" || r.Holes[0].Path != plansPath || r.Holes[0].Line != 12 {
		t.Errorf("holes = %+v", r.Holes)
	}
	if len(r.Usecases) != 1 || r.Usecases[0].Name != "growth.ReadPlan" || !slices.Equal(r.Usecases[0].Details, []string{"ports Ledger, Clock", "in ReadPlanInput", "out Plan"}) {
		t.Errorf("use cases = %+v", r.Usecases)
	}
	if got := names(r.Adapters); !slices.Equal(got, []string{"postgres.Plans", "vendor.Stream"}) {
		t.Fatalf("adapters = %+v", r.Adapters)
	}
	if a := r.Adapters[1]; a.Path != streamPath || !slices.Equal(a.Details, []string{"port growth.Ledger"}) || a.Note != "handwritten: the vendor API streams a chunked body" {
		t.Errorf("vendor.Stream = %+v", a)
	}
	if !slices.Equal(r.Adapters[0].Details, []string{"port growth.Ledger"}) {
		t.Errorf("postgres.Plans = %+v", r.Adapters[0])
	}
	if !slices.Equal(r.Contracts, []string{"api/openapi.yaml", "db/queries/plans.sql", portsPath}) {
		t.Errorf("contracts = %v", r.Contracts)
	}
	if len(r.Layers) == 0 || r.Layers[0] != "port internal/application/{area}" {
		t.Errorf("layers = %v", r.Layers)
	}
}

func TestBuildWithoutAManifestParsesTheUseCases(t *testing.T) {
	t.Parallel()
	in := genmap.Input{Snapshot: growthSnapshot(t, map[string]string{"internal/application/growth/read_plan.go": readPlan, streamPath: stream}), Policy: model.DefaultGenerate()}
	r, err := genmap.Build(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	if got := names(r.Usecases); !slices.Equal(got, []string{"growth.ReadPlan"}) {
		t.Errorf("use cases = %v", got)
	}
	if got := names(r.Adapters); !slices.Equal(got, []string{"vendor.Stream"}) {
		t.Errorf("adapters = %v", got)
	}
	if !slices.Equal(r.Contracts, []string{"api/openapi.yaml"}) {
		t.Errorf("contracts = %v", r.Contracts)
	}
}

func TestBuildKeepsOneArea(t *testing.T) {
	t.Parallel()
	extra := map[string]string{plansPath: ownedPlans}
	growth, err := genmap.Build(t.Context(), genmap.Input{Snapshot: growthSnapshot(t, maps.Clone(extra)), Manifest: growthManifest(), Policy: model.DefaultGenerate(), Area: "growth"})
	if err != nil || len(growth.Ports) != 2 || len(growth.Holes) != 1 || len(growth.Usecases) != 1 {
		t.Fatalf("growth = %+v, %v", growth, err)
	}
	billing, err := genmap.Build(t.Context(), genmap.Input{Snapshot: growthSnapshot(t, extra), Manifest: growthManifest(), Policy: model.DefaultGenerate(), Area: "billing"})
	if err != nil || len(billing.Ports) != 0 || len(billing.Holes) != 0 || len(billing.Usecases) != 0 || len(billing.Adapters) != 0 {
		t.Fatalf("billing = %+v, %v", billing, err)
	}
}

// Red row: a marked file that is no port, use case, adapter or handler is
// still listed, with its reason, in the text and the JSON. Near row: a
// marked adapter keeps its note on its item and is not listed twice.
func TestBuildListsEveryHandwrittenFileWithItsReason(t *testing.T) {
	t.Parallel()
	const chunkPath = "internal/adapters/out/vendor/chunk.go"
	chunk := "// golaw:handwritten the vendor splits a body in chunks\npackage vendor\n\n// Chunk is one part of a body.\ntype Chunk []byte\n"
	in := genmap.Input{Snapshot: growthSnapshot(t, map[string]string{streamPath: stream, chunkPath: chunk}), Manifest: growthManifest(), Policy: model.DefaultGenerate()}
	r, err := genmap.Build(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	want := []genmap.Item{{Name: chunkPath, Path: chunkPath, Note: "the vendor splits a body in chunks"}}
	if !slices.EqualFunc(r.Handwritten, want, func(a, b genmap.Item) bool { return a.Path == b.Path && a.Note == b.Note }) {
		t.Fatalf("handwritten = %+v, want %+v", r.Handwritten, want)
	}
	if text := r.Text(genmap.DefaultBudget); !strings.Contains(text, "handwritten "+chunkPath+": the vendor splits a body in chunks\n") ||
		strings.Contains(text, "handwritten "+streamPath+":") {
		t.Errorf("text:\n%s", text)
	}
	data, err := r.JSON()
	if err != nil || !strings.Contains(string(data), `"handwritten": [`) || !strings.Contains(string(data), chunkPath) {
		t.Errorf("JSON = %s, %v", data, err)
	}
}
