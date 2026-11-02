package usecase_test

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

// TestGoldenSliceUsecase proves two use cases of one area,
// the first asking for both ports it consumes, green by construction.
func TestGoldenSliceUsecase(t *testing.T) {
	t.Parallel()
	slice := goldentest.Slice{
		Name: "usecase", Module: module, Base: gentest.Contracts(),
		Kinds: []genkind.Kind{support.Kind(), port.Kind(), usecase.Kind(), wiring.Kind()},
		Commands: []goldentest.Command{
			{Kind: genkind.KindUsecase, Request: readPlan("Ledger", "Clock")},
			{Kind: genkind.KindUsecase, Request: genkind.Request{Name: genkind.Name{Area: "growth", Type: "ArchivePlan"}, Ports: []string{"Ledger"}, In: "AccountID", Out: "Plan"}},
		},
		Fills: fills(t, filepath.Join("testdata", "fills.yml"), filepath.Join("..", "wiring", "testdata", "fills.yml")),
	}
	report := goldentest.Run(t, slice, goldentest.StepBuild, goldentest.StepLint, goldentest.StepScan, goldentest.StepSurface,
		goldentest.StepCoverage, goldentest.StepDup, goldentest.StepCheck, goldentest.StepFormat, goldentest.StepLaws, goldentest.StepFloor)
	for _, path := range []string{
		area + "zz_generated_read_plan.go", area + "zz_generated_archive_plan.go", area + "zz_generated_usecases.go",
		area + "zz_generated_usecases_test.go", area + "read_plan.go", area + "read_plan_test.go", area + "archive_plan.go",
		area + "archive_plan_test.go", area + "ledgertest/zz_generated_fake.go", area + "clocktest/zz_generated_fake.go",
	} {
		if len(report.Files[path]) == 0 {
			t.Errorf("the slice wrote no %s", path)
		}
	}
	for step, reason := range report.Skipped {
		t.Logf("step %s skipped: %s", step, reason)
	}
	if _, skipped := report.Skipped[goldentest.StepCoverage]; !skipped && (report.Coverage < 80 || !report.OpenCovered) {
		t.Errorf("new code coverage %.1f, open covered %v", report.Coverage, report.OpenCovered)
	}
	if _, skipped := report.Skipped[goldentest.StepDup]; !skipped && report.Duplication >= 1.5 {
		t.Errorf("duplication %.2f percent, over the golden 1.5", report.Duplication)
	}
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

// TestGoldenSliceUsecaseOverNewPorts proves a use case whose ports its area
// does not declare yet: the stubs golaw writes for them, their methods hole
// open, pass the build and the rendered lint asset before the agent
// declares a method, as make ci runs them right after golaw new.
func TestGoldenSliceUsecaseOverNewPorts(t *testing.T) {
	t.Parallel()
	report := goldentest.Run(t, goldentest.Slice{
		Name: "usecase over new ports", Module: module, Base: gentest.Contracts(),
		Kinds: []genkind.Kind{support.Kind(), port.Kind(), usecase.Kind(), wiring.Kind()},
		Commands: []goldentest.Command{
			{Kind: genkind.KindUsecase, Request: genkind.Request{Name: genkind.Name{Area: "audit", Type: "ReadEntry"}, Ports: []string{"Store", "Clock"}}},
		},
	}, goldentest.StepLint, goldentest.StepFormat, goldentest.StepFloor)
	if len(report.Files["internal/application/audit/ports.go"]) == 0 {
		t.Errorf("the slice wrote no ports.go for the new ports: %v", report.Files)
	}
}
