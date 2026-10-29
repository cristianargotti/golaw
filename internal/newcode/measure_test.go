package newcode_test

import (
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/newcode"
)

// headHits is what the line model reads from the pipeline report of a
// change that appended five one-line functions to value.go: the two old
// lines of Value, the five new return lines with the last one or two
// uncovered, a package the scope excludes and a generated file.
func headHits(fourCovered bool) map[string]newcode.LineHits {
	return map[string]newcode.LineHits{
		"value.go":            {5: true, 6: true, 10: true, 14: true, 18: true, 22: fourCovered, 26: false},
		"integration/flow.go": {4: false},
		"gen.pb.go":           {6: false},
	}
}

func headLines() map[string][]int {
	added := make([]int, 0, 20)
	for line := 8; line <= 27; line++ {
		added = append(added, line)
	}
	return map[string][]int{"value.go": added, "integration/flow.go": {1, 2, 3, 4, 5}, "notes.go": {1}}
}

func inScope(path string) bool { return path != "integration/flow.go" && path != "gen.pb.go" }

func lineRange(first, last int) []int {
	out := make([]int, 0, last-first+1)
	for line := first; line <= last; line++ {
		out = append(out, line)
	}
	return out
}

type measureCase struct {
	name    string
	in      newcode.Input
	kind    model.StatusKind
	toCover int
	covered int
	passed  bool
	files   []newcode.File
}

func measureCases() []measureCase {
	fourOfFive := []newcode.File{{Path: "value.go", ToCover: 5, Covered: 4, Uncovered: []int{26}}}
	return []measureCase{
		{
			"red-below-the-sonar-way-threshold",
			newcode.Input{Lines: headHits(false), NewLines: headLines(), Universe: inScope},
			model.StatusMeasured, 5, 3, false,
			[]newcode.File{{Path: "value.go", ToCover: 5, Covered: 3, Uncovered: []int{22, 26}}},
		},
		{
			"near-at-the-threshold",
			newcode.Input{Lines: headHits(true), NewLines: headLines(), Universe: inScope},
			model.StatusMeasured, 5, 4, true, fourOfFive,
		},
		{
			"red-the-policy-threshold",
			newcode.Input{Lines: headHits(true), NewLines: headLines(), Universe: inScope, Threshold: 90},
			model.StatusMeasured, 5, 4, false, fourOfFive,
		},
		{
			"red-without-a-universe-every-file-counts",
			newcode.Input{Lines: headHits(true), NewLines: headLines()},
			model.StatusMeasured, 6, 4, false,
			[]newcode.File{
				{Path: "integration/flow.go", ToCover: 1, Uncovered: []int{4}},
				{Path: "value.go", ToCover: 5, Covered: 4, Uncovered: []int{26}},
			},
		},
		{
			"near-a-line-named-twice-counts-once",
			newcode.Input{Lines: headHits(true), NewLines: map[string][]int{"value.go": {14, 10, 10}}, Universe: inScope},
			model.StatusMeasured, 2, 2, true,
			[]newcode.File{{Path: "value.go", ToCover: 2, Covered: 2}},
		},
		{
			"near-no-new-line-to-cover",
			newcode.Input{Lines: headHits(false), NewLines: map[string][]int{"value.go": {1, 2, 3, 7}}, Universe: inScope},
			model.StatusNotApplicable, 0, 0, false, nil,
		},
		{
			"red-the-report-names-no-file-of-the-universe",
			newcode.Input{Lines: map[string]newcode.LineHits{"gen.pb.go": {6: false}}, NewLines: headLines(), Universe: inScope},
			model.StatusUnmeasured, 0, 0, false, nil,
		},
		{
			"red-an-empty-report",
			newcode.Input{Lines: map[string]newcode.LineHits{}, NewLines: headLines(), Universe: inScope},
			model.StatusUnmeasured, 0, 0, false, nil,
		},
	}
}

func TestMeasureCountsTheNewLinesToCover(t *testing.T) {
	t.Parallel()
	for _, tc := range measureCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := newcode.Measure(tc.in)
			if got.Status.Kind != tc.kind || got.ToCover != tc.toCover || got.Covered != tc.covered ||
				got.Passed() != tc.passed || !reflect.DeepEqual(got.Files, tc.files) {
				t.Fatalf("Measure = %+v", got)
			}
		})
	}
}

func TestMeasureAppliesTheSonarWayThresholdByDefault(t *testing.T) {
	t.Parallel()
	in := newcode.Input{Lines: headHits(false), NewLines: headLines(), Universe: inScope}
	if got := newcode.Measure(in); got.Threshold != model.DefaultCoverageNew || got.Percent() != 60 {
		t.Fatalf("default = %d, %v", got.Threshold, got.Percent())
	}
	in.Threshold = 90
	if got := newcode.Measure(in).Threshold; got != 90 {
		t.Fatalf("threshold = %d, want the policy's 90", got)
	}
	if got := (newcode.Report{}).Percent(); got != 0 {
		t.Fatalf("an empty report = %v, want 0, never 100", got)
	}
}

func TestReportTextIsWhatAPersonReads(t *testing.T) {
	t.Parallel()
	below := "verdict: below the threshold, cover the uncovered new lines"
	for _, tc := range []struct {
		name   string
		report newcode.Report
		want   []string
	}{
		{"red", newcode.Measure(newcode.Input{Lines: headHits(false), NewLines: headLines(), Universe: inScope}), []string{
			"new code: 60.0% (3/5 new lines covered), threshold 80%", "value.go: 3/5 covered, uncovered lines 22, 26", below,
		}},
		{"near", newcode.Measure(newcode.Input{Lines: headHits(true), NewLines: headLines(), Universe: inScope}), []string{
			"new code: 80.0% (4/5 new lines covered), threshold 80%", "value.go: 4/5 covered, uncovered lines 26", "verdict: at or above the threshold",
		}},
		{"near-a-fully-covered-file", newcode.Measure(newcode.Input{Lines: headHits(true), NewLines: map[string][]int{"value.go": {10}}}), []string{
			"new code: 100.0% (1/1 new lines covered), threshold 80%", "value.go: 1/1 covered", "verdict: at or above the threshold",
		}},
		{"unmeasured", newcode.Measure(newcode.Input{Lines: map[string]newcode.LineHits{}, NewLines: headLines()}), []string{
			"new code: unmeasured, the report names no file of the coverage universe",
		}},
		{"not-applicable", newcode.Measure(newcode.Input{Lines: headHits(true), NewLines: map[string][]int{}}), []string{
			"new code: not applicable, no new line to cover",
		}},
		{"red-twenty-lines-and-a-count", newcode.Report{Threshold: 80, ToCover: 25, Status: model.Measured(), Files: []newcode.File{
			{Path: "big.go", ToCover: 25, Uncovered: lineRange(1, 25)},
		}}, []string{
			"new code: 0.0% (0/25 new lines covered), threshold 80%",
			"big.go: 0/25 covered, uncovered lines 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, and 5 more",
			below,
		}},
		{"red-never-rounds-up-to-the-threshold", newcode.Report{Threshold: 80, ToCover: 10000, Covered: 7999, Status: model.Measured()}, []string{
			"new code: 79.9% (7999/10000 new lines covered), threshold 80%", below,
		}},
		{"red-a-report-without-a-status", newcode.Report{}, []string{"new code: unmeasured, the report carries no status"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.report.Text(); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Text = %q, want %q", got, tc.want)
			}
		})
	}
}
