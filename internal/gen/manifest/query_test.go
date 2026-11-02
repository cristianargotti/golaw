package manifest_test

import (
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/manifest"
)

func decoded(t *testing.T) manifest.Manifest {
	t.Helper()
	m, err := manifest.Decode([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestLookupOwnerRegenerableAndContracting(t *testing.T) {
	t.Parallel()
	m := decoded(t)
	if e, ok := m.Lookup("usecase", "growth.ReadPlan"); !ok || e.Out != "Plan" {
		t.Errorf("Lookup = %+v, %v", e, ok)
	}
	if _, ok := m.Lookup("usecase", "growth.ArchivePlan"); ok {
		t.Error("Lookup of an absent name answered true")
	}
	tests := map[string]struct {
		path        string
		owner       string
		regenerable bool
	}{
		"red: a regenerable file":             {path: "internal/adapters/out/postgres/zz_generated_plans.go", owner: "postgres.Plans", regenerable: true},
		"red: an owned file":                  {path: "internal/application/growth/read_plan.go", owner: "growth.ReadPlan"},
		"near: a file of no entry":            {path: "internal/adapters/out/postgres/helpers.go"},
		"near: the contract is not an output": {path: "db/queries/plans.sql"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e, ok := m.Owner(tc.path)
			if (tc.owner != "") != ok || ok && e.Name != tc.owner {
				t.Errorf("Owner = %+v, %v; want %q", e, ok, tc.owner)
			}
			if _, ok := m.Regenerable(tc.path); ok != tc.regenerable {
				t.Errorf("Regenerable = %v, want %v", ok, tc.regenerable)
			}
		})
	}
	if got := m.Contracting("db/queries/plans.sql"); len(got) != 1 || got[0].Name != "postgres.Plans" {
		t.Errorf("Contracting = %+v", got)
	}
	if got := m.Contracting("api/openapi.yaml"); len(got) != 0 {
		t.Errorf("Contracting of no contract = %+v", got)
	}
}

func TestWithAddsOrReplacesAndKeepsTheOrder(t *testing.T) {
	t.Parallel()
	m := decoded(t)
	port := manifest.Entry{Kind: "port", Template: "port@1", Name: "growth.Ledger"}
	added := m.With(port)
	if len(added.Entries) != 3 || added.Entries[1].Kind != "port" || len(m.Entries) != 2 {
		t.Fatalf("With added = %+v, original %d entries", added.Entries, len(m.Entries))
	}
	if err := added.Validate(); err != nil {
		t.Errorf("Validate after With = %v", err)
	}
	changed := m.Entries[1]
	changed.Ports = []string{"Ledger"}
	replaced := m.With(changed)
	if len(replaced.Entries) != 2 || !reflect.DeepEqual(replaced.Entries[1].Ports, []string{"Ledger"}) {
		t.Errorf("With replaced = %+v", replaced.Entries)
	}
	if e := m.Entries[0]; e.Area() != "postgres" || e.Type() != "Plans" {
		t.Errorf("Area, Type = %q, %q", e.Area(), e.Type())
	}
}
