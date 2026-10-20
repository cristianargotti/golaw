package coverage_test

import (
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/coverage"
)

func TestLinesRenderFailuresAndSkippedUnits(t *testing.T) {
	t.Parallel()
	total := coverage.Measure{Name: "total", Statements: 20, Covered: 15}
	pkg := coverage.Measure{Name: moduleName, Statements: 20, Covered: 15}
	file := coverage.Measure{Name: "value.go", Statements: 20, Covered: 15}
	missing := coverage.Measure{Name: "missing.go"}
	small := coverage.Measure{Name: "small.go", Statements: 2, Covered: 1}
	report := coverage.Report{
		Total: total, Packages: []coverage.Measure{pkg},
		Files:    []coverage.Measure{file, missing, small},
		Failures: []coverage.Measure{total, pkg, file, missing},
		Skipped:  []coverage.Measure{small},
	}
	want := []string{
		"total: 75.0% (15/20)",
		"below the floor: total: 75.0% (15/20)",
		"below the floor: example.test/sample: 75.0% (15/20)",
		"below the floor: value.go: 75.0% (15/20)",
		"file missing from the profile: missing.go",
		"under the statement minimum: small.go: 50.0% (1/2)",
	}
	if got := report.Lines(); !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %#v", got)
	}
}

func TestLinesKeepAnEmptyMeasuredUniverseClean(t *testing.T) {
	t.Parallel()
	report := coverage.Report{Total: coverage.Measure{Name: "total"}}
	want := []string{"total: 100.0% (0/0)"}
	if got := report.Lines(); !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %#v", got)
	}
}
