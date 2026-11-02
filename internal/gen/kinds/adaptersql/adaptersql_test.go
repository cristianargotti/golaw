package adaptersql_test

import (
	"bytes"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/contract"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adaptersql"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
)

const (
	sliceModule = "example.com/slice"
	portsPath   = "internal/application/growth/ports.go"
	dir         = "internal/adapters/out/postgres"
)

const (
	readConstraint    = "call p.queries.ReadPlan with the parameters converted to its types; answer planOf of its row and error; wrap the error with gensupport.Prefix(actionPlansReadPlan)"
	archiveConstraint = "call p.queries.ArchivePlan with the parameters converted to its types; map 0 affected rows to growth.ErrPlanNotFound; wrap the error with gensupport.Prefix(actionPlansArchivePlan)"
	mapperConstraint  = "copy the columns of row into growth.Plan; map sql.ErrNoRows to growth.ErrPlanNotFound; answer every other error as it is"
	seedConstraint    = "open the integration database of the repository and answer ledgertest.Subject{Port: postgres.NewPlans(db), Seed: insert the rows each case's Args and Want need}"
)

func portEntry() manifest.Entry {
	return manifest.Entry{
		Kind: genkind.KindPort, Template: "port@1", Name: "growth.Ledger",
		Contracts: []manifest.Contract{{Path: portsPath, Symbol: "Ledger"}},
	}
}

func input(t *testing.T, files map[string][]byte, entries ...manifest.Entry) genkind.Input {
	t.Helper()
	m := manifest.Manifest{Schema: manifest.Schema, Support: manifest.Support{Path: "internal/gensupport", Template: "support@1"}}
	for i := range entries {
		m = m.With(entries[i])
	}
	snap := gentest.Snapshot(t, files)
	return genkind.Input{
		Module: sliceModule, Version: "0.3.0", Policy: model.DefaultGenerate(),
		Snapshot: snap, Manifest: m, Contracts: contract.Reader{Snapshot: snap},
	}
}

func request(typ string) genkind.Request {
	return genkind.Request{Name: genkind.Name{Area: "postgres", Type: typ}, Port: genkind.Name{Area: "growth", Type: "Ledger"}}
}

func newAdapter(t *testing.T, files map[string][]byte, typ string, entries ...manifest.Entry) genkind.Output {
	t.Helper()
	out, err := adaptersql.Kind().New(t.Context(), input(t, files, append([]manifest.Entry{portEntry()}, entries...)...), request(typ))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func byPath(files []genkind.File) map[string][]byte {
	out := make(map[string][]byte, len(files))
	for _, f := range files {
		out[f.Path] = f.Content
	}
	return out
}

// written answers files with every file of out at its path.
func written(files map[string][]byte, out genkind.Output) map[string][]byte {
	next := maps.Clone(files)
	maps.Copy(next, byPath(out.Regenerable))
	maps.Copy(next, byPath(out.Owned))
	return next
}

func openHoles(t *testing.T, files map[string][]byte) map[string]string {
	t.Helper()
	found := make(map[string]string)
	for p, content := range files {
		list, err := holes.Find(p, content)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		for _, h := range list {
			found[h.ID] = h.Constraint
		}
	}
	return found
}

func TestKindNamesItself(t *testing.T) {
	t.Parallel()
	if k := adaptersql.Kind(); k.Name() != genkind.KindAdapterSQL || k.Template() != 1 {
		t.Errorf("Kind = %s@%d", k.Name(), k.Template())
	}
}

func TestNewWritesTheSQLAdapterShape(t *testing.T) {
	t.Parallel()
	out := newAdapter(t, gentest.Contracts(), "Plans")
	regenerable, owned := byPath(out.Regenerable), byPath(out.Owned)
	wantRegenerable := []string{dir + "/zz_generated_plans.go", dir + "/zz_generated_plans_contract_test.go"}
	wantOwned := []string{dir + "/doc.go", dir + "/plans.go", dir + "/plans_integration_test.go", dir + "/plans_internal_test.go", dir + "/plans_test.go"}
	if got := slices.Sorted(maps.Keys(regenerable)); !slices.Equal(got, wantRegenerable) {
		t.Errorf("regenerable = %v", got)
	}
	if got := slices.Sorted(maps.Keys(owned)); !slices.Equal(got, wantOwned) {
		t.Errorf("owned = %v", got)
	}
	entry := out.Entry
	wantContracts := []manifest.Contract{{Path: portsPath, Symbol: "Ledger"}, {Path: "db/queries/plans.sql"}}
	if entry.Kind != genkind.KindAdapterSQL || entry.Template != "adapter-sql@1" || entry.Name != "postgres.Plans" ||
		!slices.Equal(entry.Regenerable, wantRegenerable) || !slices.Equal(entry.Owned, wantOwned) || !slices.Equal(entry.Contracts, wantContracts) {
		t.Errorf("entry = %+v", entry)
	}
	for p, content := range regenerable {
		if h, ok := genheader.Parse(content); !ok || h.Kind != genkind.KindAdapterSQL || h.Template != 1 || h.Version != "0.3.0" {
			t.Errorf("%s header = %+v, %v", p, h, ok)
		}
	}
	for p, content := range owned {
		if _, ok := genheader.Parse(content); ok {
			t.Errorf("owned %s carries the golaw header", p)
		}
	}
	wantBindings := []genkind.Binding{{
		Kind: genkind.KindAdapterSQL, Area: "growth", Port: "Ledger", Import: genkind.Import{Path: sliceModule + "/" + dir},
		Call: "postgres.NewPlans(infra.DB)", Needs: []genkind.InfraUse{{Field: "DB", Type: "*sql.DB", Import: genkind.Import{Path: "database/sql"}}},
	}}
	if !reflect.DeepEqual(out.Bindings, wantBindings) || len(out.Appends) != 0 || len(out.Decisions) != 0 || len(out.Needs) != 0 {
		t.Errorf("bindings %+v, appends %+v, decisions %+v, needs %+v", out.Bindings, out.Appends, out.Decisions, out.Needs)
	}
}

func TestNewRendersThinRegenerableFiles(t *testing.T) {
	t.Parallel()
	regenerable := byPath(newAdapter(t, gentest.Contracts(), "Plans").Regenerable)
	store := string(regenerable[dir+"/zz_generated_plans.go"])
	for _, want := range []string{
		"\npackage postgres\n",
		`"` + sliceModule + `/internal/adapters/out/postgres/sqlcgen"`,
		"actionPlansReadPlan    = \"postgres: plans read plan\"",
		"actionPlansArchivePlan = \"postgres: plans archive plan\"",
		"type Plans struct {\n\tqueries *sqlcgen.Queries\n}",
		"func NewPlans(db *sql.DB) *Plans {\n\treturn &Plans{queries: sqlcgen.New(db)}\n}",
		"var _ growth.Ledger = (*Plans)(nil)",
	} {
		if !strings.Contains(store, want) {
			t.Errorf("zz_generated_plans.go lacks %q", want)
		}
	}
	suite := string(regenerable[dir+"/zz_generated_plans_contract_test.go"])
	for _, want := range []string{"\n//go:build integration\n\npackage postgres_test\n", "ledgertest.Run(t, plansSubject)", "t.Parallel()"} {
		if !strings.Contains(suite, want) {
			t.Errorf("zz_generated_plans_contract_test.go lacks %q", want)
		}
	}
}

func TestNewOpensOneHoleWithItsRedTestPerMethod(t *testing.T) {
	t.Parallel()
	owned := byPath(newAdapter(t, gentest.Contracts(), "Plans").Owned)
	want := map[string]string{
		"plans.read-plan": readConstraint, "plans.archive-plan": archiveConstraint,
		"plans.plan-of": mapperConstraint, "seed.plans": seedConstraint,
	}
	if got := openHoles(t, owned); !maps.Equal(got, want) {
		t.Errorf("holes = %v, want %v", got, want)
	}
	stub := "// ReadPlan reads the plan of one account.\n" +
		"func (p *Plans) ReadPlan(ctx context.Context, account growth.AccountID) (growth.Plan, error) {\n" +
		"\t// golaw:hole plans.read-plan " + readConstraint + "\n" +
		"\tvar zero growth.Plan\n" +
		"\treturn zero, gensupport.Hole(\"plans.read-plan\", actionPlansReadPlan, p.queries, ctx, account)\n}\n"
	mapper := "func planOf(row sqlcgen.Plan, err error) (growth.Plan, error) {\n" +
		"\t// golaw:hole plans.plan-of " + mapperConstraint + "\n" +
		"\treturn gensupport.HoleOf[growth.Plan](\"plans.plan-of\", row, err)\n}\n"
	red := "func TestPlansReadPlan(t *testing.T) {\n\tt.Parallel()\n" +
		"\tdb := sqltest.Open(t, sqltest.Step{Err: sql.ErrNoRows})\n\tvar account growth.AccountID\n" +
		"\t_, err := postgres.NewPlans(db).ReadPlan(t.Context(), account)\n" +
		"\tif errors.Is(err, gensupport.ErrNotImplemented) {\n\t\tt.Fatal(\"hole plans.read-plan is open: " + readConstraint + "\")\n\t}\n}\n"
	tests := []struct{ path, want string }{
		{dir + "/plans.go", stub},
		{dir + "/plans.go", mapper},
		{dir + "/plans_test.go", red},
		{dir + "/plans_internal_test.go", "if _, err := planOf(row, nil); errors.Is(err, gensupport.ErrNotImplemented) {"},
		{dir + "/plans_integration_test.go", "//go:build integration\n\npackage postgres_test\n"},
		{dir + "/plans_integration_test.go", "\t\tt.Fatalf(\"hole seed.plans is open: " + seedConstraint + ": %v\", err)\n"},
		{dir + "/doc.go", "// Package postgres holds the outbound adapters of its directory.\npackage postgres\n"},
	}
	for _, tc := range tests {
		if !bytes.Contains(owned[tc.path], []byte(tc.want)) {
			t.Errorf("%s lacks:\n%s\ngot:\n%s", tc.path, tc.want, owned[tc.path])
		}
	}
}
