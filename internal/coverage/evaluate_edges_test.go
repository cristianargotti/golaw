package coverage_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/coverage"
)

func TestFileExemptionsRetainOtherFloors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		exemptions []string
		minimum    int
		want       []string
		skipped    int
	}{
		{"exact", []string{"value.go"}, 0, []string{"total", moduleName}, 0},
		{"prefix", []string{"value"}, 0, []string{"total", moduleName, "value.go"}, 0},
		{"pattern", []string{"*.go"}, 0, []string{"total", moduleName, "value.go"}, 0},
		{"small", nil, 21, []string{"total"}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			floors := coverage.DefaultFloors()
			floors.Exemptions = tc.exemptions
			floors.MinStatements = tc.minimum
			profile := "mode: set\n" + moduleName + "/value.go:4.2,23.10 20 0\n"
			report, err := coverage.Evaluate(strings.NewReader(profile), coverageRoot(t), floors)
			if err != nil {
				t.Fatal(err)
			}
			failures := make([]string, 0, len(report.Failures))
			for _, failure := range report.Failures {
				failures = append(failures, failure.Name)
			}
			if !reflect.DeepEqual(failures, tc.want) || len(report.Skipped) != tc.skipped ||
				report.Total.Statements != 20 || report.Total.Covered != 0 {
				t.Fatalf("report = %+v", report)
			}
		})
	}
}

func TestMissingExemptSourceStillFails(t *testing.T) {
	t.Parallel()
	floors := coverage.DefaultFloors()
	floors.Exemptions = []string{"value.go"}
	report, err := coverage.Evaluate(strings.NewReader("mode: atomic\n"), coverageRoot(t), floors)
	want := []coverage.Measure{{Name: "value.go"}}
	if err != nil || !reflect.DeepEqual(report.Files, want) ||
		!reflect.DeepEqual(report.Failures, want) || len(report.Skipped) != 0 ||
		report.Total != (coverage.Measure{Name: "total"}) {
		t.Fatalf("report = %+v, error = %v", report, err)
	}
}

func TestEmptyBuildUniverseHasNoMissingMeasurements(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCoverageFile(t, root, "go.mod", "module "+moduleName+"\n")
	report, err := coverage.Evaluate(strings.NewReader("mode: atomic\n"), root, coverage.DefaultFloors())
	if err != nil || report.Total.Percent() != 100 || len(report.Failures) != 0 ||
		len(report.Files) != 0 || len(report.Packages) != 0 || len(report.Skipped) != 0 {
		t.Fatalf("report = %+v, error = %v", report, err)
	}
}

func TestFailuresFollowSortedScopes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCoverageFile(t, root, "go.mod", "module "+moduleName+"\n")
	for _, name := range []string{"z/z.go", "a/z.go", "a/a.go"} {
		writeCoverageFile(t, root, name, "package sample\nfunc Value() int { return 1 }\n")
	}
	profile := "mode: atomic\n" + moduleName + "/z/z.go:2.1,2.36 20 0\n" +
		moduleName + "/a/z.go:2.1,2.36 20 0\n"
	report, err := coverage.Evaluate(strings.NewReader(profile), root, coverage.DefaultFloors())
	if err != nil {
		t.Fatal(err)
	}
	failures := make([]string, 0, len(report.Failures))
	for _, failure := range report.Failures {
		failures = append(failures, failure.Name)
	}
	want := []string{"total", moduleName + "/a", moduleName + "/z", "a/a.go", "a/z.go", "z/z.go"}
	if !reflect.DeepEqual(failures, want) {
		t.Fatalf("failures = %v", failures)
	}
}

func TestTotalStatementOverflowIsNotAResult(t *testing.T) {
	t.Parallel()
	root := coverageRoot(t)
	writeCoverageFile(t, root, "second.go", "package sample\nfunc Second() int { return 1 }\n")
	const maximum = int(^uint(0) >> 1)
	profile := fmt.Sprintf("mode: atomic\n%s/value.go:4.2,23.10 %d 1\n%s/second.go:2.1,2.37 1 1\n",
		moduleName, maximum, moduleName)
	_, err := coverage.Evaluate(strings.NewReader(profile), root, coverage.DefaultFloors())
	if !errors.Is(err, coverage.ErrProfile) {
		t.Fatalf("overflow error = %v", err)
	}
}

func TestThresholdUsesUnroundedPercent(t *testing.T) {
	t.Parallel()
	profile := "mode: atomic\n" + moduleName + "/value.go:4.2,19.5 1599 1\n" +
		moduleName + "/value.go:20.2,23.10 401 0\n"
	report, err := coverage.Evaluate(strings.NewReader(profile), coverageRoot(t), coverage.DefaultFloors())
	if err != nil || len(report.Failures) != 3 || report.Total.Percent() >= 80 {
		t.Fatalf("report = %+v, error = %v", report, err)
	}
}
