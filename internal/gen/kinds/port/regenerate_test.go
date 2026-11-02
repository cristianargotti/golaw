package port_test

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/port"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/gentest"
)

const casesGo = `package ledgertest

// readPlanCases are the cases of ReadPlan.
func readPlanCases() []Case {
	return nil
}

// archivePlanCases are the cases of ArchivePlan.
func archivePlanCases() []Case {
	return nil
}
`

const orphan = `
// forgetPlanCases are the cases of a method the port dropped.
func forgetPlanCases() []Case {
	return nil
}
`

func TestRegenerateOfAnUnchangedPortTouchesNoOwnedCode(t *testing.T) {
	t.Parallel()
	in := input(t, withFile(ledgerDir+"cases.go", casesGo), []manifest.Entry{ledgerEntry()}, ledger())
	out, err := port.Kind().Regenerate(t.Context(), in, ledgerEntry())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out.Entry, ledgerEntry()) || !slices.Equal(paths(out.Regenerable), ledgerEntry().Regenerable) {
		t.Errorf("entry %+v, regenerable %v", out.Entry, paths(out.Regenerable))
	}
	if len(out.Owned) != 0 || len(out.Appends) != 0 || len(out.Decisions) != 0 || len(out.Needs) != 0 {
		t.Errorf("owned %v, appends %+v, decisions %+v, needs %+v", paths(out.Owned), out.Appends, out.Decisions, out.Needs)
	}
}

func TestRegenerateAppendsTheCasesOfANewMethod(t *testing.T) {
	t.Parallel()
	grown := ledger()
	grown.Methods = append(grown.Methods, genkind.Method{
		Name: "PurgePlan", Line: 16,
		Params: []genkind.Param{{Name: "ctx", Type: "context.Context"}}, Results: []genkind.Param{{Type: "error"}},
	})
	in := input(t, withFile(ledgerDir+"cases.go", casesGo), []manifest.Entry{ledgerEntry()}, grown)
	out, err := port.Kind().Regenerate(t.Context(), in, ledgerEntry())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Appends) != 1 || out.Appends[0].Path != ledgerDir+"cases.go" || !slices.Equal(out.Appends[0].Holes, []string{"ledger.purge-plan"}) {
		t.Fatalf("appends = %+v", out.Appends)
	}
	assertHas(t, string(out.Appends[0].Decls[0]), "func purgePlanCases() []Case {", "// golaw:hole ledger.purge-plan")
	assertHas(t, content(t, out.Regenerable, ledgerDir+"zz_generated_suite.go"), "slices.Concat(readPlanCases(), archivePlanCases(), purgePlanCases())")
}

func TestRegenerateNamesTheCasesOfAMethodThatIsGone(t *testing.T) {
	t.Parallel()
	in := input(t, withFile(ledgerDir+"cases.go", casesGo+orphan), []manifest.Entry{ledgerEntry()}, ledger())
	out, err := port.Kind().Regenerate(t.Context(), in, ledgerEntry())
	if err != nil {
		t.Fatal(err)
	}
	want := []genkind.Decision{{
		Path: ledgerDir + "cases.go", Line: 14,
		Text: "forgetPlanCases drives ForgetPlan, which is no longer in port Ledger: delete it or restore the method in internal/application/growth/ports.go",
	}}
	if !reflect.DeepEqual(out.Decisions, want) {
		t.Errorf("decisions = %+v, want %+v", out.Decisions, want)
	}
}

func TestRegenerateWritesTheCasesFileAgainWhenItIsGone(t *testing.T) {
	t.Parallel()
	out, err := port.Kind().Regenerate(t.Context(), input(t, gentest.Contracts(), []manifest.Entry{ledgerEntry()}, ledger()), ledgerEntry())
	if err != nil {
		t.Fatal(err)
	}
	if got := paths(out.Owned); !slices.Equal(got, []string{ledgerDir + "cases.go"}) || len(out.Appends) != 0 {
		t.Errorf("owned %v, appends %+v", got, out.Appends)
	}
}

func TestRegenerateOfAPortNoLongerDeclared(t *testing.T) {
	t.Parallel()
	files := withFile(portsPath, "package growth\n\nimport \"time\"\n\n// Clock tells the time.\ntype Clock interface {\n\t// Now answers the time.\n\tNow() time.Time\n}\n")
	out, err := port.Kind().Regenerate(t.Context(), input(t, files, []manifest.Entry{ledgerEntry()}), ledgerEntry())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Decisions) != 1 || len(out.Regenerable) != 0 ||
		!strings.Contains(out.Decisions[0].Text, "port growth.Ledger is no longer declared in internal/application/growth/ports.go") {
		t.Errorf("decisions %+v, regenerable %v", out.Decisions, paths(out.Regenerable))
	}
}

func TestRegenerateRefusesAnEntryWithoutItsContract(t *testing.T) {
	t.Parallel()
	entry := ledgerEntry()
	entry.Contracts = nil
	if _, err := port.Kind().Regenerate(t.Context(), input(t, gentest.Contracts(), nil), entry); !errors.Is(err, genkind.ErrRequest) {
		t.Errorf("Regenerate = %v, want ErrRequest", err)
	}
}
