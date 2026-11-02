package port_test

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/goldentest"
	"github.com/cristianargotti/golaw/internal/gen/kinds/port"
	"github.com/cristianargotti/golaw/internal/gen/kinds/support"
	"github.com/cristianargotti/golaw/internal/gen/kinds/usecase"
	"github.com/cristianargotti/golaw/internal/gen/kinds/wiring"
	"github.com/cristianargotti/golaw/internal/gentest"
)

// TestGoldenSlicePort proves two port instances, one through
// the use case's need and one through golaw new port, green by
// construction: built, linted, scanned, covered, without duplication,
// checked and formatted, holes open and then filled.
func TestGoldenSlicePort(t *testing.T) {
	t.Parallel()
	slice := goldentest.Slice{
		Name: "port", Module: module, Base: gentest.Contracts(),
		Kinds: []genkind.Kind{support.Kind(), port.Kind(), usecase.Kind(), wiring.Kind()},
		Commands: []goldentest.Command{
			{Kind: genkind.KindUsecase, Request: genkind.Request{Name: genkind.Name{Area: "growth", Type: "ReadPlan"}, Ports: []string{"Ledger"}, In: "AccountID", Out: "Plan"}},
			{Kind: genkind.KindPort, Request: genkind.Request{Name: genkind.Name{Area: "growth", Type: "Clock"}, Usecase: "ReadPlan"}},
		},
		Fills: fills(t, filepath.Join("testdata", "fills.yml"), filepath.Join("..", "wiring", "testdata", "fills.yml")),
	}
	report := goldentest.Run(t, slice, goldentest.StepBuild, goldentest.StepLint, goldentest.StepScan, goldentest.StepSurface,
		goldentest.StepCoverage, goldentest.StepDup, goldentest.StepCheck, goldentest.StepFormat, goldentest.StepLaws, goldentest.StepFloor)
	for _, path := range []string{
		ledgerDir + "zz_generated_fake.go", ledgerDir + "zz_generated_suite.go", ledgerDir + "zz_generated_suite_test.go", ledgerDir + "cases.go",
		area + "clocktest/zz_generated_fake.go", area + "clocktest/cases.go", area + "zz_generated_usecases.go", area + "read_plan.go",
		"internal/gensupport/porttest/zz_generated_suite.go", ".golaw/gen.yml",
	} {
		if len(report.Files[path]) == 0 {
			t.Errorf("the slice wrote no %s", path)
		}
	}
	assertGreen(t, report)
}

// fills joins the fill files of the slice: the composition root the
// wiring kind writes holds a hole of its own the kind's fills do not close.
func fills(t *testing.T, paths ...string) []goldentest.Fill {
	t.Helper()
	out := make([]goldentest.Fill, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var list []goldentest.Fill
		if err := yaml.Unmarshal(data, &list); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		out = append(out, list...)
	}
	return out
}

// assertGreen holds the numbers of M7.10.3 a run measured: a step the
// harness skipped is named with its reason and never passes silently.
func assertGreen(t *testing.T, r goldentest.Report) {
	t.Helper()
	for step, reason := range r.Skipped {
		t.Logf("step %s skipped: %s", step, reason)
	}
	if _, skipped := r.Skipped[goldentest.StepCoverage]; !skipped && (r.Coverage < 80 || !r.OpenCovered) {
		t.Errorf("new code coverage %.1f with the fills, every regenerable line covered with the holes open: %v", r.Coverage, r.OpenCovered)
	}
	if _, skipped := r.Skipped[goldentest.StepDup]; !skipped && r.Duplication >= 1.5 {
		t.Errorf("duplication %.2f percent, the golden threshold is 1.5", r.Duplication)
	}
}
