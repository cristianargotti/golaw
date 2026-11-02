package handler_test

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/goldentest"
	"github.com/cristianargotti/golaw/internal/gen/kinds/handler"
	"github.com/cristianargotti/golaw/internal/gen/kinds/port"
	"github.com/cristianargotti/golaw/internal/gen/kinds/support"
	"github.com/cristianargotti/golaw/internal/gen/kinds/usecase"
	"github.com/cristianargotti/golaw/internal/gen/kinds/wiring"
	"github.com/cristianargotti/golaw/internal/gentest"
)

// fillFiles are the recorded fills of every kind the slice runs, each kept
// beside its kind by the kind's owner; a later file wins for an id both hold.
func fillFiles() []string {
	return []string{
		filepath.Join("..", "port", "testdata", "fills.yml"),
		filepath.Join("..", "usecase", "testdata", "fills.yml"),
		filepath.Join("..", "wiring", "testdata", "fills.yml"),
		filepath.Join("testdata", "fills.yml"),
	}
}

// fillFile is a recorded fills file: the top-level list every kind writes
// and goldentest.LoadFills reads.
type fillFile []struct {
	ID      string `yaml:"id"`
	Body    string `yaml:"body"`
	Imports []struct {
		Name string `yaml:"name"`
		Path string `yaml:"path"`
	} `yaml:"imports"`
}

func loadFills(t *testing.T, paths ...string) []goldentest.Fill {
	t.Helper()
	byID := map[string]goldentest.Fill{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("the recorded fills %s are missing: every kind the slice runs records the fills of its holes: %v", path, err)
		}
		var file fillFile
		if err := yaml.Unmarshal(data, &file); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, f := range file {
			fill := goldentest.Fill{ID: f.ID, Body: f.Body}
			for _, i := range f.Imports {
				fill.Imports = append(fill.Imports, genkind.Import{Name: i.Name, Path: i.Path})
			}
			byID[f.ID] = fill
		}
	}
	fills := make([]goldentest.Fill, 0, len(byID))
	for _, id := range slices.Sorted(maps.Keys(byID)) {
		fills = append(fills, byID[id])
	}
	return fills
}

// sliceCommands builds the growth slice: the two use cases over the fixture
// types, whose needs create the fakes of their ports (a second golaw new
// port of either would be ErrExists), then the handler over the area.
func sliceCommands() []goldentest.Command {
	return []goldentest.Command{
		{Kind: genkind.KindUsecase, Request: genkind.Request{
			Name: genkind.Name{Area: "growth", Type: "ReadPlan"}, Ports: []string{"Ledger", "Clock"}, In: "AccountID", Out: "Plan",
		}},
		{Kind: genkind.KindUsecase, Request: genkind.Request{
			Name: genkind.Name{Area: "growth", Type: "ArchivePlan"}, Ports: []string{"Ledger"}, In: "AccountID", Out: "Plan",
		}},
		{Kind: genkind.KindHandler, Request: genkind.Request{
			Name: genkind.Name{Area: "plansapi"}, API: "api/openapi.yaml", Usecases: []string{"growth"},
		}},
	}
}

func TestGoldenSliceHandler(t *testing.T) {
	if testing.Short() {
		t.Skip("the golden slice builds a scratch module: run make golden-gen")
	}
	t.Parallel()
	report := goldentest.Run(t, goldentest.Slice{
		Name: genkind.KindHandler, Module: "example.com/slice", Base: gentest.Contracts(),
		Kinds:    []genkind.Kind{support.Kind(), port.Kind(), usecase.Kind(), handler.Kind(), wiring.Kind()},
		Commands: sliceCommands(), Fills: loadFills(t, fillFiles()...),
	}, goldentest.StepBuild, goldentest.StepLint, goldentest.StepScan, goldentest.StepSurface, goldentest.StepCoverage,
		goldentest.StepDup, goldentest.StepCheck, goldentest.StepFormat, goldentest.StepLaws, goldentest.StepFloor)
	for step, reason := range report.Skipped {
		t.Logf("step %s skipped: %s", step, reason)
	}
	for _, path := range []string{
		"internal/adapters/in/plansapi/zz_generated_handler.go", "internal/adapters/in/plansapi/zz_generated_handler_routes_test.go",
		"internal/adapters/in/plansapi/plans.go", "internal/adapters/in/plansapi/status.go",
	} {
		if _, ok := report.Files[path]; !ok {
			t.Errorf("the slice wrote no %s", path)
		}
	}
	if _, skipped := report.Skipped[goldentest.StepCoverage]; !skipped && (report.Coverage < 80 || !report.OpenCovered) {
		t.Errorf("coverage on new code %.1f with the fills, every regenerable line covered with the holes open %v", report.Coverage, report.OpenCovered)
	}
	if _, skipped := report.Skipped[goldentest.StepDup]; !skipped && report.Duplication >= 1.5 {
		t.Errorf("duplication on new lines %.2f percent, the golden threshold is 1.5 (A50)", report.Duplication)
	}
}
