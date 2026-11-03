package check_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/check"
)

func numbered(n int) []string {
	lines := make([]string, 0, n)
	for i := range n {
		lines = append(lines, fmt.Sprintf("a.go:%d: defect", i+1))
	}
	return lines
}

func TestReportLines(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		report  check.Report
		verbose bool
		want    []string
	}{
		{
			"red-a-gate-over-the-bound-counts-the-rest",
			check.Report{Base: "origin/main", Results: []check.Result{check.Fail("lint", numbered(check.MaxLines+3)...), check.Pass("vet")}},
			false,
			append(append([]string{"golaw check: 2 gates, 1 green, 1 red, 0 skipped (base origin/main)", "red lint:"},
				prefixed(numbered(check.MaxLines))...), "  ... and 3 more"),
		},
		{
			"near-a-gate-at-the-bound-prints-every-line",
			check.Report{Base: "main", Results: []check.Result{check.Fail("test", numbered(check.MaxLines)...)}},
			false,
			append([]string{"golaw check: 1 gates, 0 green, 1 red, 0 skipped (base main)", "red test:"}, prefixed(numbered(check.MaxLines))...),
		},
		{
			"near-green-gates-print-only-when-verbose",
			check.Report{Whole: true, Note: "no base", Results: []check.Result{check.Pass("vet"), check.Skip("migrations", "the db capability is off")}},
			true,
			[]string{"golaw check: 2 gates, 1 green, 0 red, 1 skipped (whole tree)", "no base", "green vet", "skipped migrations: the db capability is off"},
		},
		{
			"near-quiet-green",
			check.Report{Base: "main", Results: []check.Result{check.Pass("vet")}},
			false,
			[]string{"golaw check: 1 gates, 1 green, 0 red, 0 skipped (base main)"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.report.Lines(tc.verbose); !slices.Equal(got, tc.want) {
				t.Fatalf("lines =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
			if red := tc.report.Count(check.Red) > 0; tc.report.Red() != red {
				t.Errorf("Red = %v", tc.report.Red())
			}
		})
	}
}

func prefixed(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, "  "+line)
	}
	return out
}

// Run begins every gate before it waits for any and answers in gate order,
// a gate of two results included, never stopping at a red one.
func TestRunBeginsEveryGateBeforeItWaits(t *testing.T) {
	t.Parallel()
	var begun []string
	gate := func(name string, results ...check.Result) check.Gate {
		return func(context.Context) check.Wait {
			begun = append(begun, name)
			return func() []check.Result {
				if len(begun) != 3 {
					return []check.Result{check.Fail(name, "waited before every gate began")}
				}
				return results
			}
		}
	}
	results := check.Run(t.Context(), []check.Gate{
		gate("one", check.Fail("one", "red")),
		gate("two", check.Pass("two"), check.Skip("two-after", "follows")),
		func(context.Context) check.Wait {
			begun = append(begun, "three")
			return check.Done(check.Pass("three"))
		},
	})
	names := make([]string, 0, len(results))
	for _, result := range results {
		names = append(names, result.Gate+"="+string(result.Status))
	}
	want := []string{"one=red", "two=green", "two-after=skipped", "three=green"}
	if !slices.Equal(names, want) {
		t.Fatalf("results = %v, want %v", names, want)
	}
}
