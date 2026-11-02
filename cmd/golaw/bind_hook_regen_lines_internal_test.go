package main

import (
	"fmt"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/apply"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestRegenLinesSpellTheAgentAndThePersonForms(t *testing.T) {
	t.Parallel()
	o := regenOutcome{
		contracts: []string{"internal/application/growth/ports.go"},
		labels:    map[string]string{"internal/application/growth/ports.go": " (port Ledger)"},
		written:   3,
		result: gen.Result{
			Open:      []holes.Hole{{ID: "plans.archive-plan", Constraint: "call p.queries.ArchivePlan", Path: "internal/adapters/out/postgres/plans.go", Line: 88}},
			Closed:    []holes.Hole{{ID: "plans.read-plan", Path: "internal/adapters/out/postgres/plans.go"}},
			Decisions: []genkind.Decision{{Path: "internal/adapters/out/postgres/plans.go", Line: 120, Text: "(*Plans).PurgePlans is no longer in port Ledger: delete it or restore the method in internal/application/growth/ports.go"}},
		},
		skipped: []apply.Skip{{Path: "internal/application/growth/ledgertest/zz_generated_fake.go", Cause: "changed on disk since the hook read it"}},
		causes:  []string{"sqlc passed the hook's 1.5 s limit"},
	}
	report := regenLines(model.DefaultLanguage, o)
	want := []string{
		"golaw regenerated from internal/application/growth/ports.go (port Ledger): 3 files, 1 drift",
		"hole open internal/adapters/out/postgres/plans.go:88 plans.archive-plan call p.queries.ArchivePlan",
		"hole closed internal/adapters/out/postgres/plans.go plans.read-plan",
		"decision internal/adapters/out/postgres/plans.go:120 (*Plans).PurgePlans is no longer in port Ledger: delete it or restore the method in internal/application/growth/ports.go",
		"skipped internal/application/growth/ledgertest/zz_generated_fake.go: changed on disk since the hook read it",
	}
	if !slices.Equal(report.Lines, want) {
		t.Fatalf("lines =\n%q\nwant\n%q", report.Lines, want)
	}
	if !slices.Equal(report.Notices, []string{"not regenerated in the hook: run golaw gen (sqlc passed the hook's 1.5 s limit)"}) {
		t.Fatalf("notices = %q", report.Notices)
	}
	person := []string{
		"golaw regenerou a partir de internal/application/growth/ports.go (port Ledger): 3 arquivos, 1 com deriva",
		"lacuna aberta internal/adapters/out/postgres/plans.go:88 plans.archive-plan call p.queries.ArchivePlan",
		"lacuna fechada internal/adapters/out/postgres/plans.go plans.read-plan",
		"decisão internal/adapters/out/postgres/plans.go:120 (*Plans).PurgePlans is no longer in port Ledger: delete it or restore the method in internal/application/growth/ports.go",
		"arquivo pulado internal/application/growth/ledgertest/zz_generated_fake.go: changed on disk since the hook read it",
		"golaw não regenerou no hook: rode golaw gen (sqlc passed the hook's 1.5 s limit).",
	}
	if !slices.Equal(report.Person, person) {
		t.Fatalf("person =\n%q\nwant\n%q", report.Person, person)
	}
	if english := regenLines("en", o); !slices.Equal(english.Person, append(slices.Clone(english.Lines), english.Notices...)) {
		t.Fatalf("english person = %q", english.Person)
	}
}

func TestRegenLinesBoundEachFileAtTwentyLines(t *testing.T) {
	t.Parallel()
	open := make([]holes.Hole, 0, 25)
	for i := range 25 {
		open = append(open, holes.Hole{ID: fmt.Sprintf("plans.m%02d", i), Constraint: "x", Path: "internal/adapters/out/postgres/plans.go", Line: i + 1})
	}
	report := regenLines(model.DefaultLanguage, regenOutcome{result: gen.Result{Open: open}})
	if len(report.Lines) != 21 || report.Lines[20] != "internal/adapters/out/postgres/plans.go: 5 more lines; golaw gen --check names them all" {
		t.Fatalf("lines = %q", report.Lines)
	}
}

// The drift of a generated file a shell command wrote names its contract,
// and the person reads its cause in pt-BR.
func TestDriftLinesNameTheContractInBothForms(t *testing.T) {
	t.Parallel()
	const fake = "internal/application/growth/clocktest/zz_generated_fake.go"
	o := regenOutcome{
		drift:     []gen.Drift{{Path: fake, Cause: gen.DriftDiffers, Fix: gen.FixGen}},
		contracts: nil, driftContracts: map[string]string{fake: "internal/application/growth/ports.go"},
	}
	report := regenLines(model.DefaultLanguage, o)
	agent := "drift " + fake + " differs from its rendering: edit internal/application/growth/ports.go, or run golaw gen"
	person := "deriva " + fake + " difere da sua renderização: edite internal/application/growth/ports.go, ou rode golaw gen"
	if !slices.Equal(report.Lines, []string{agent}) || !slices.Equal(report.Person, []string{person}) {
		t.Fatalf("lines %q, person %q", report.Lines, report.Person)
	}
	bare := regenLines(model.DefaultLanguage, regenOutcome{drift: []gen.Drift{{Path: fake, Cause: "a cause of another kind"}}})
	if !slices.Equal(bare.Person, []string{"deriva " + fake + " a cause of another kind: rode golaw gen"}) {
		t.Fatalf("person = %q", bare.Person)
	}
}

// A contract that declares two ports labels both in the summary.
func TestContractLabelsNameEveryPortOfTheContract(t *testing.T) {
	t.Parallel()
	const ports = "internal/application/growth/ports.go"
	port := func(symbol string) manifest.Entry {
		return manifest.Entry{Kind: genkind.KindPort, Template: "port@1", Name: "growth." + symbol, Contracts: []manifest.Contract{{Path: ports, Symbol: symbol}}}
	}
	m := manifest.Manifest{Schema: manifest.Schema, Entries: []manifest.Entry{port("Ledger"), port("Clock")}}
	if got := contractLabels(m, []string{ports}); got[ports] != " (ports Ledger, Clock)" {
		t.Errorf("label = %q", got[ports])
	}
	if got := contractLabels(manifest.Manifest{Schema: manifest.Schema, Entries: []manifest.Entry{port("Ledger")}}, []string{ports}); got[ports] != " (port Ledger)" {
		t.Errorf("label = %q", got[ports])
	}
}
