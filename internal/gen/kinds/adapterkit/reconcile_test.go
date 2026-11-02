package adapterkit_test

import (
	"slices"
	"testing"
	"testing/fstest"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adapterkit"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
)

const ownedPlans = `// Package postgres holds the outbound adapters of its directory.
package postgres

// ReadPlan reads the plan of one account.
func (p *Plans) ReadPlan() error { return nil }

func planOf() {}

// PurgeAll purges every plan.
func (p *Plans) PurgeAll() error { return nil }

func (p *Plans) scan() {}
`

func TestScanAndReconcileNameTheOrphans(t *testing.T) {
	t.Parallel()
	files := gentest.Contracts()
	files["internal/adapters/out/postgres/plans.go"] = []byte(ownedPlans)
	files["internal/adapters/out/postgres/other.go"] = []byte("package postgres\n\n// Extra is written by hand.\nfunc (p *Plans) Extra() error { return nil }\n")
	files["internal/adapters/out/postgres/plans_test.go"] = []byte("package postgres_test\n\nfunc helper() {}\n")
	target := plansTarget(t, files)
	pkg, err := adapterkit.Scan(gentest.Snapshot(t, files), target.Dir, "Plans")
	if err != nil {
		t.Fatal(err)
	}
	if !pkg.Doc || pkg.Funcs["planOf"].Line != 7 || pkg.Methods["PurgeAll"].Line != 10 || len(pkg.Methods) != 4 {
		t.Errorf("Scan = %+v", pkg)
	}
	if _, ok := pkg.Funcs["helper"]; ok {
		t.Error("Scan read a test file")
	}
	methods, _, err := adapterkit.Methods(target)
	if err != nil {
		t.Fatal(err)
	}
	missing, decisions := adapterkit.Reconcile(target, methods, pkg, []string{"internal/adapters/out/postgres/plans.go"})
	if len(missing) != 1 || missing[0].Name != "ArchivePlan" {
		t.Errorf("missing = %+v", missing)
	}
	want := []genkind.Decision{{
		Path: "internal/adapters/out/postgres/plans.go", Line: 10,
		Text: "(*Plans).PurgeAll is no longer in port Ledger: delete it or restore the method in internal/application/growth/ports.go",
	}}
	if !slices.Equal(decisions, want) {
		t.Errorf("decisions = %+v, want %+v", decisions, want)
	}
}

func TestSentinelsAndTheirSentence(t *testing.T) {
	t.Parallel()
	target := plansTarget(t, gentest.Contracts())
	got, err := adapterkit.Sentinels(gentest.Snapshot(t, gentest.Contracts()), target.Port, "NotFound")
	if err != nil || !slices.Equal(got, []string{"growth.ErrPlanNotFound"}) {
		t.Fatalf("Sentinels = %v, %v", got, err)
	}
	tests := []struct {
		name      string
		sentinels []string
		want      string
	}{
		{"one", []string{"growth.ErrPlanNotFound"}, "growth.ErrPlanNotFound"},
		{"several", []string{"growth.ErrA", "growth.ErrB"}, "one of growth.ErrA, growth.ErrB"},
		{"none", nil, "the port's not found sentinel, declared once in internal/application/growth/errors.go"},
	}
	for _, tc := range tests {
		if s := adapterkit.Sentence(tc.sentinels, "internal/application/growth"); s != tc.want {
			t.Errorf("%s: Sentence = %q, want %q", tc.name, s, tc.want)
		}
	}
}

func TestDeclRendersOneFormattedDeclaration(t *testing.T) {
	t.Parallel()
	templates := fstest.MapFS{
		"stub.go.tmpl":   {Data: []byte("func {{.Name}}() error {\nreturn nil\n}\n")},
		"broken.go.tmpl": {Data: []byte("func {{.Name}}( {\n")},
	}
	got, err := adapterkit.Decl(templates, "stub.go.tmpl", struct{ Name string }{"Ping"})
	if err != nil || string(got) != "func Ping() error {\n\treturn nil\n}\n" {
		t.Errorf("Decl = %q, %v", got, err)
	}
	if _, err := adapterkit.Decl(templates, "stub.go.tmpl", struct{ Other string }{"x"}); err == nil {
		t.Error("a missing key rendered")
	}
	if _, err := adapterkit.Decl(templates, "broken.go.tmpl", struct{ Name string }{"Ping"}); err == nil {
		t.Error("a declaration that does not parse rendered")
	}
}

func TestBindingsGoToTheFirstAdapterOfAPort(t *testing.T) {
	t.Parallel()
	port := []manifest.Contract{{Path: portsPath, Symbol: "Ledger"}}
	sqlEntry := manifest.Entry{Kind: genkind.KindAdapterSQL, Template: "adapter-sql@1", Name: "postgres.Plans", Contracts: port}
	httpEntry := manifest.Entry{Kind: genkind.KindAdapterHTTP, Template: "adapter-http@1", Name: "planapi.Client", Contracts: port}
	b := genkind.Binding{Kind: genkind.KindAdapterSQL, Area: "growth", Port: "Ledger"}
	base := manifest.Manifest{Schema: manifest.Schema}.With(portEntry())
	if got := adapterkit.Bindings(base, sqlEntry, b); len(got) != 1 {
		t.Errorf("a lone adapter binds %d times", len(got))
	}
	if got := adapterkit.Bindings(base.With(httpEntry), sqlEntry, b); len(got) != 0 {
		t.Errorf("a second adapter of the port bound itself: %+v", got)
	}
	if got := adapterkit.Bindings(base.With(sqlEntry), httpEntry, b); len(got) != 1 {
		t.Errorf("the first adapter in manifest order lost its binding: %+v", got)
	}
}
