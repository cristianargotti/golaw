package gen_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
)

func TestGenerateRemovesAnEntryWhoseOwnedFilesAreGone(t *testing.T) {
	t.Parallel()
	m := enrolled(plansEntry())
	files := inSync(t, m)
	delete(files, plansPath)
	res, err := gen.Generate(t.Context(), engineInput(t, files, m, supportKind(), regenerating()))
	if err != nil {
		t.Fatal(err)
	}
	if w := writeOf(t, res, plansGen); w.Mode != gen.WriteDelete || w.Base == "" {
		t.Errorf("the regenerable file = %+v, want a delete over the bytes the run read", w)
	}
	for _, w := range res.Writes {
		if w.Path == plansPath {
			t.Error("the owned file the person deleted was written again")
		}
	}
	if _, kept := res.Manifest.Lookup(genkind.KindAdapterSQL, "postgres.Plans"); kept || len(res.Removed) != 1 || res.Removed[0].Name != "postgres.Plans" {
		t.Errorf("removed = %+v, the manifest keeps the entry: %v", res.Removed, kept)
	}
	check, err := gen.Check(t.Context(), engineInput(t, files, m, supportKind(), regenerating()))
	if err != nil || !slices.ContainsFunc(check.Drift, func(d gen.Drift) bool {
		return d.Path == plansGen && d.Cause == gen.DriftRemoved && d.Fix == gen.FixGen
	}) {
		t.Errorf("check = %+v, %v, want the removal named as drift", check.Drift, err)
	}
}

func TestGenerateKeepsAnEntryWhoseOwnedFilesRemain(t *testing.T) {
	t.Parallel()
	m := enrolled(plansEntry())
	files := inSync(t, m)
	delete(files, plansGen)
	res, err := gen.Generate(t.Context(), engineInput(t, files, m, supportKind(), regenerating()))
	if err != nil || len(res.Removed) != 0 || writeOf(t, res, plansGen).Mode != gen.WriteRegenerable {
		t.Errorf("a deleted regenerable file = %+v, removed %+v, %v, want it written again", res.Writes, res.Removed, err)
	}
	delete(files, plansPath)
	in := engineInput(t, files, m, supportKind(), regenerating())
	in.Changed = []string{portsPath}
	if res, err := gen.Generate(t.Context(), in); err != nil || len(res.Removed) != 0 {
		t.Errorf("a run over changed paths removed %+v, %v: only a full run removes", res.Removed, err)
	}
}

func TestGenerateKeepsAPortItsUseCaseStillLists(t *testing.T) {
	t.Parallel()
	port := manifest.Entry{
		Kind: genkind.KindPort, Template: "port@1", Name: "growth.Ledger",
		Contracts: []manifest.Contract{{Path: portsPath, Symbol: "Ledger"}},
		Owned:     []string{"internal/application/growth/ledgertest/cases.go"},
	}
	use := manifest.Entry{
		Kind: genkind.KindUsecase, Template: "usecase@1", Name: "growth.ReadPlan", Ports: []string{"Ledger"},
		Owned: []string{"internal/application/growth/read_plan.go"},
	}
	files := gentest.Contracts()
	files["internal/application/growth/read_plan.go"] = []byte("package growth\n")
	kinds := []genkind.Kind{
		supportKind(),
		fakeKind{name: genkind.KindPort, regen: genkind.Output{Entry: port}},
		fakeKind{name: genkind.KindUsecase, regen: genkind.Output{Entry: use}},
	}
	res, err := gen.Generate(t.Context(), engineInput(t, files, enrolled(port, use), kinds...))
	if err != nil || len(res.Removed) != 0 {
		t.Errorf("removed = %+v, %v: a port its use case lists stays and its cases file is written again", res.Removed, err)
	}
}

func TestAGoneContractNamesTheRemovalRoute(t *testing.T) {
	t.Parallel()
	m := enrolled(plansEntry())
	files := inSync(t, m)
	delete(files, portsPath)
	res, err := gen.Generate(t.Context(), engineInput(t, files, m, supportKind(), regenerating()))
	want := "delete the owned files " + plansPath + " and run golaw gen to remove the entry"
	if err != nil || len(res.Decisions) != 1 || !strings.Contains(res.Decisions[0].Text, want) {
		t.Errorf("decisions = %+v, %v, want one naming %q", res.Decisions, err, want)
	}
}
