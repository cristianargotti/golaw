package lint

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/surfacefix"
)

// The web expectations of the twin under the strict set: each one at its
// count in the red twin, none of them in the corrected twin.
func TestWebTwinYieldsTheExpectedFindings(t *testing.T) {
	t.Parallel()
	red := webTwin(t, surfacefix.Red)
	corrected := webTwin(t, surfacefix.Corrected)
	for _, d := range webDrivers() {
		var expected []surfacefix.Expectation
		for _, e := range surfacefix.Expectations() {
			if e.Check == "lint."+d.Tool {
				expected = append(expected, e)
			}
		}
		if len(expected) == 0 {
			t.Fatalf("the twin expects nothing of %s", d.Tool)
		}
		t.Run(d.Tool, func(t *testing.T) {
			t.Parallel()
			path := webToolPath(t, d.Tool)
			dir := webMaterialize(t, d, model.SurfaceStrict)
			got := webTally(webIssues(t, d, path, dir, model.SurfaceStrict, red))
			fixed := webTally(webIssues(t, d, path, dir, model.SurfaceStrict, corrected))
			assertWebTwin(t, expected, got, fixed)
		})
	}
}

// TestTheCorrectedTwinIsCleanUnderTheStrictWebSet: under strict the
// corrected twin yields no web finding at all, expected or not, so the set
// init selects never reds a tree the kit itself calls clean.
func TestTheCorrectedTwinIsCleanUnderTheStrictWebSet(t *testing.T) {
	t.Parallel()
	corrected := webTwin(t, surfacefix.Corrected)
	for _, d := range webDrivers() {
		t.Run(d.Tool, func(t *testing.T) {
			t.Parallel()
			dir := webMaterialize(t, d, model.SurfaceStrict)
			for _, issue := range webIssues(t, d, webToolPath(t, d.Tool), dir, model.SurfaceStrict, corrected) {
				t.Errorf("%s:%d %s/%s %s", issue.Path, issue.Line, issue.Tool, issue.Rule, issue.Message)
			}
		})
	}
}
