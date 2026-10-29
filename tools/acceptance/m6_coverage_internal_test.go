package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestM6JudgeNumberReadsTheExitAndThePercent(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		got  result
		code int
		pass bool
	}{
		{"near the parity number under the floor", result{code: 1, stdout: "new code: 63.6% (7/11)\n"}, 1, true},
		{"red another number", result{code: 1, stdout: "new code: 81.8% (9/11)\n"}, 1, false},
		{"red the exit of a pass", result{code: 0, stdout: "new code: 63.6% (7/11)\n"}, 1, false},
		{"red the process failed", result{err: ErrAcceptance}, 1, false},
	} {
		if err := m6JudgeNumber(row.got, row.code, m6ParityNumber); (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
	}
}

func TestM6JudgeLabelsWantsTwoDifferentProfiles(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		got  result
		pass bool
	}{
		{"near two numbers", result{stdout: m6StrictLabel + "\ntotal: 100.0% (9/9)\n" + m6ParityLabel + "\ntotal: 88.9% (8/9)\n"}, true},
		{"near the other order", result{code: 1, stdout: m6ParityLabel + "\ntotal: 88.9% (8/9)\n" + m6StrictLabel + "\ntotal: 100.0% (9/9)\n"}, true},
		{"red the same numbers", result{stdout: m6StrictLabel + "\ntotal: 88.9% (8/9)\n" + m6ParityLabel + "\ntotal: 88.9% (8/9)\n"}, false},
		{"red no parity label", result{stdout: m6StrictLabel + "\ntotal: 100.0% (9/9)\n"}, false},
		{"red exit 2", result{code: 2, stderr: "golaw: profile unreadable\n"}, false},
	} {
		if err := m6JudgeLabels(row.got); (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
	}
	for _, row := range []struct {
		name string
		got  result
		pass bool
	}{
		{"near unmeasured with its cause", result{code: 2, stderr: "golaw: profile.cov is older than a/new.go: unmeasured\n"}, true},
		{"red exit 0", result{code: 0, stdout: "new code: 100.0% (11/11)\n"}, false},
		{"red exit 2 without a cause", result{code: 2}, false},
	} {
		if err := m6JudgeUnmeasuredReport(row.got); (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
	}
}

// m6CoverageReplies answer go test by writing the report it names, and
// coverage by the numbers the fixture records, recording every go test.
func m6CoverageReplies(t *testing.T, s *m6Suite, tests *[][]string) []m5Reply {
	t.Helper()
	return []m5Reply{
		m6InitReply(),
		{program: s.goTool, prefix: []string{"test"}, answer: func(t *testing.T, c command) result {
			t.Helper()
			*tests = append(*tests, c.args)
			at := slices.Index(c.args, "-coverprofile")
			checkedWrite(t, filepath.Join(c.dir, c.args[at+1]), "mode: set\n")
			return result{}
		}},
		{prefix: []string{m6CoverageCommand}, answer: func(_ *testing.T, c command) result {
			switch {
			case slices.Contains(c.args, "absent.cov"):
				return result{code: 2, stderr: "golaw: absent.cov: no such report: unmeasured\n"}
			case slices.Contains(c.args, m6PlainInputFlag):
				return result{code: 1, stdout: m6StrictLabel + "\nb/b.go: 100.0% (1/1)\n" + m6ParityLabel + "\nb/b.go: 0.0% (0/1)\n"}
			case slices.Contains(c.args, m6StrictReport):
				return result{stdout: "new code: " + m6StrictNumber + " (9/11)\n"}
			default:
				return result{code: 1, stdout: "new code: " + m6ParityNumber + " (7/11)\n"}
			}
		}},
	}
}

func TestM6CoverageRowsRunTheMeasuredPackages(t *testing.T) {
	t.Parallel()
	for _, c := range m6CoverageCases() {
		s := m6TestSuite(t)
		var tests [][]string
		s.execute = m5Script(t, s.suite, m6CoverageReplies(t, s, &tests)...).Execute
		err := c.run(s.m6Scope(m6CoverageCategory, c.name), t.Context())
		if c.name == "stale-report" {
			// The simulated candidate answers the parity number, not the
			// refusal a stale report earns, so the row must fail here.
			if err == nil {
				t.Errorf("%s passed over a candidate that measured a stale report", c.name)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
		for _, args := range tests {
			if !strings.Contains(strings.Join(args, " "), strings.Join(m6Packages(), " ")) ||
				(slices.Contains(args, "-coverprofile") && args[slices.Index(args, "-coverprofile")+1] == m6StrictReport) != slices.Contains(args, "-coverpkg="+strings.Join(m6Packages(), ",")) {
				t.Errorf("%s ran go %q", c.name, args)
			}
		}
	}
}
