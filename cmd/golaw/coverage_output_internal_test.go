package main

import (
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/coverage"
)

func TestCoverageLabelsTheTwoUniverses(t *testing.T) {
	t.Parallel()
	local := coverage.Report{Total: coverage.Measure{Name: "total", Statements: 20, Covered: 18}}
	plain := coverage.Report{Total: coverage.Measure{Name: "total", Statements: 20, Covered: 15}}
	for _, tc := range []struct {
		name  string
		plain *coverage.Report
		want  []string
	}{
		{"near-without-a-plain-profile-the-output-stays", nil, []string{
			"profile --input", "total: 90.0% (18/20)", "profile --plain-input: not given",
		}},
		{"red-both-numbers-are-labeled", &plain, []string{
			"strict floors: profile --input", "total: 90.0% (18/20)",
			"pipeline parity: profile --plain-input", "total: 75.0% (15/20)",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := coverageLines(local, tc.plain); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("lines = %q, want %q", got, tc.want)
			}
		})
	}
}
