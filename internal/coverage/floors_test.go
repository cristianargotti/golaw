package coverage_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/coverage"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestFloorsFromPolicyOptions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   model.Options
		want coverage.Floors
	}{
		{"near-defaults", nil, coverage.DefaultFloors()},
		{
			"red-independent-values",
			model.Options{
				"total":           model.IntValue(85),
				"package":         model.IntValue(90),
				"file":            model.IntValue(95),
				"min_statements":  model.IntValue(0),
				"file_exemptions": model.StringsValue("a.go"),
			},
			coverage.Floors{
				Total: 85, Package: 90, File: 95,
				MinStatements: 0, Exemptions: []string{"a.go"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := coverage.FloorsFrom(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("floors = %+v", got)
			}
			if len(got.Exemptions) != 0 {
				got.Exemptions[0] = "changed.go"
				if again := coverage.FloorsFrom(tc.in); again.Exemptions[0] != "a.go" {
					t.Fatal("floors mutated policy options")
				}
			}
		})
	}
}

func TestMinimumAndExemptionsKeepTheTotal(t *testing.T) {
	t.Parallel()
	profile := "mode: atomic\n" +
		moduleName + "/value.go:4.2,23.10 19 0\n"
	for _, tc := range []struct {
		name    string
		floors  coverage.Floors
		fail    int
		skipped int
	}{
		{
			"red-total-enforced",
			coverage.Floors{Total: 80, Package: 80, File: 80, MinStatements: 20},
			1, 2,
		},
		{
			"near-file-exemption-only",
			coverage.Floors{
				Total: 80, Package: 80, File: 80,
				Exemptions: []string{"value.go"},
			},
			2, 0,
		},
		{
			"red-exemption-is-exact",
			coverage.Floors{
				Total: 80, Package: 80, File: 80,
				Exemptions: []string{"value"},
			},
			3, 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			report, err := coverage.Evaluate(
				strings.NewReader(profile), coverageRoot(t), tc.floors,
			)
			if err != nil || len(report.Failures) != tc.fail ||
				len(report.Skipped) != tc.skipped ||
				report.Total.Statements != 19 {
				t.Fatalf("report = %+v, error = %v", report, err)
			}
		})
	}
}
