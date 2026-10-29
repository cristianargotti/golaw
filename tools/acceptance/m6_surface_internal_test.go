package main

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/surfacefix"
)

// m6SurfaceReplies answer scan --json and lint --surface --json with the
// documents of the red twin when red, clean ones otherwise, and record the
// data home each lint ran with.
func m6SurfaceReplies(t *testing.T, red bool, data *[]string) []m5Reply {
	t.Helper()
	exps := surfacefix.Expectations()
	scan, lint := result{stdout: m6Doc(t, nil)}, result{stdout: m6Doc(t, nil)}
	if red {
		scan = result{code: 1, stdout: m6Doc(t, m6Found(exps, false))}
		lint = result{code: 1, stdout: m6Doc(t, m6Found(exps, true))}
	}
	return []m5Reply{
		m6InitReply(),
		{prefix: []string{commandScan, m2JSONFlag}, answer: m5Answer(scan)},
		{prefix: []string{m6LintCommand, m6SurfaceFlag, m2JSONFlag}, answer: func(_ *testing.T, c command) result {
			*data = append(*data, environmentValue(c.env, envDataHome))
			return lint
		}},
	}
}

func TestM6SurfaceRowsJudgeEachTwin(t *testing.T) {
	t.Parallel()
	for _, c := range m6SurfaceCases() {
		red := c.name == "red-scan" || c.name == "red-lint"
		for _, tc := range []struct {
			kind      string
			redAnswer bool
			pass      bool
		}{
			{"near the answers of its twin", red, true},
			{"red the answers of the other twin", !red, false},
		} {
			s := m6TestSuite(t)
			var data []string
			s.execute = m5Script(t, s.suite, m6SurfaceReplies(t, tc.redAnswer, &data)...).Execute
			err := c.run(s.m6Scope(m6SurfaceCategory, c.name+"-"+tc.kind), t.Context())
			if (err == nil) != tc.pass {
				t.Errorf("%s over %s: %v", c.name, tc.kind, err)
			}
			if c.tools && (len(data) != 1 || data[0] != s.shared.data) {
				t.Errorf("%s ran lint with the data homes %q, want %q", c.name, data, s.shared.data)
			}
		}
	}
}

// TestM6SurfaceRowsAreRequiredAndFirst holds the wiring: the lint rows need
// the tools, the scan rows do not, and the category comes first.
func TestM6SurfaceRowsAreRequiredAndFirst(t *testing.T) {
	t.Parallel()
	catalogs := m6Catalogs()
	if len(catalogs) == 0 || catalogs[0].category != m6SurfaceCategory ||
		!slices.Equal(m6Names(catalogs[0].cases), []string{"red-scan", "red-lint", "corrected-scan", "corrected-lint"}) {
		t.Fatalf("the first catalog = %+v", catalogs)
	}
	for _, c := range m6SurfaceCases() {
		if c.tools != (c.name == "red-lint" || c.name == "corrected-lint") {
			t.Errorf("%s needs the tools: %v", c.name, c.tools)
		}
	}
}
