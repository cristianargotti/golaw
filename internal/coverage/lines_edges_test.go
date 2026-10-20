package coverage_test

import (
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/coverage"
)

func TestLinesPreserveFailuresWhenDisplayRoundsUp(t *testing.T) {
	t.Parallel()
	measure := coverage.Measure{Name: "total", Statements: 10000, Covered: 7999}
	report := coverage.Report{Total: measure, Failures: []coverage.Measure{measure}}
	want := []string{
		"total: 80.0% (7999/10000)",
		"below the floor: total: 80.0% (7999/10000)",
	}
	if got := report.Lines(); !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %#v", got)
	}
}

func TestLinesKeepMeasuredEmptyUnitsDistinctFromMissingSources(t *testing.T) {
	t.Parallel()
	report := coverage.Report{
		Total:   coverage.Measure{Name: "total"},
		Skipped: []coverage.Measure{{Name: "zero.go"}},
	}
	want := []string{
		"total: 100.0% (0/0)",
		"under the statement minimum: zero.go: 100.0% (0/0)",
	}
	if got := report.Lines(); !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %#v", got)
	}
}
