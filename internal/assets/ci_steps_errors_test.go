package assets_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
)

func TestRunStepsDeclaredShape(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, input, reason string
		count               int
	}{
		{"empty jobs", "jobs: {}\n", "", 0},
		{"actions only", "jobs: {ci: {steps: [{uses: local/action}]}}\n", "", 0},
		{"empty steps", "jobs: {ci: {steps: []}}\n", "", 0},
		{"no name", "jobs: {ci: {steps: [{run: make}]}}\n", "", 1},
		{"irrelevant metadata", "name: [metadata]\njobs: {}\n", "", 0},
		{"broken YAML", "jobs: [\n", "decode", 0},
		{"empty document", "", "mapping", 0},
		{"sequence root", "[]\n", "mapping", 0},
		{"missing jobs", "name: CI\n", "jobs", 0},
		{"jobs sequence", "jobs: []\n", "jobs", 0},
		{"scalar job", "jobs: {ci: false}\n", "jobs.ci", 0},
		{"missing steps", "jobs: {ci: {}}\n", "steps", 0},
		{"scalar steps", "jobs: {ci: {steps: false}}\n", "steps", 0},
		{"scalar step", "jobs: {ci: {steps: [false]}}\n", "step", 0},
		{"numeric run", "jobs: {ci: {steps: [{run: 42}]}}\n", "run", 0},
		{"empty run", "jobs: {ci: {steps: [{run: \"\"}]}}\n", "run", 0},
		{"missing command", "jobs: {ci: {steps: [{name: empty}]}}\n", "run or uses", 0},
		{"both commands", "jobs: {ci: {steps: [{run: make, uses: local/action}]}}\n", "run or uses", 0},
		{"job environment", "jobs: {ci: {env: [], steps: []}}\n", "env", 0},
		{"step environment", "jobs: {ci: {steps: [{run: make, env: []}]}}\n", "env", 0},
		{"nonstring environment", "jobs: {ci: {env: {A: false}, steps: []}}\n", "env.A", 0},
		{"nonstring name", "jobs: {ci: {steps: [{name: false, run: make}]}}\n", "name", 0},
		{"duplicate jobs", "jobs: {}\njobs: {}\n", "duplicate", 0},
		{"duplicate run", "jobs: {ci: {steps: [{run: make, run: go}]}}\n", "duplicate", 0},
		{"second document", "jobs: {}\n---\njobs: {}\n", "document", 0},
		{"alias", "x: &x {steps: []}\njobs: {ci: *x}\n", "alias", 0},
		{"merge", "x: &x {A: one}\njobs: {ci: {env: {<<: *x}, steps: []}}\n", "merge", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := assets.RunSteps([]byte(test.input))
			if test.reason == "" {
				if err != nil || len(got) != test.count {
					t.Fatalf("steps=%#v, error=%v", got, err)
				}
				return
			}
			assertRunStepsRefusal(t, got, err, test.reason)
		})
	}
}

func TestRunStepsErrorLocations(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, input, location string
		line                  int
	}{
		{"empty document", "", "mapping", 1},
		{"root", "\n[]\n", "mapping", 2},
		{"missing jobs", "\nname: CI\n", "jobs", 2},
		{"missing steps", "\njobs:\n  ci: {}\n", "jobs.ci.steps", 3},
		{"run", "jobs:\n  ci:\n    steps:\n      - run: 42\n", "jobs.ci.steps.run", 4},
		{"uses", "jobs:\n  ci:\n    steps:\n      - uses: false\n", "jobs.ci.steps.uses", 4},
		{"name", "jobs:\n  ci:\n    steps:\n      - run: make\n        name: false\n", "jobs.ci.steps.name", 5},
		{"environment", "jobs:\n  ci:\n    env:\n      A: false\n    steps: []\n", "jobs.ci.env.A", 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := assets.RunSteps([]byte(test.input))
			if !errors.Is(err, assets.ErrRunSteps) ||
				!strings.Contains(err.Error(), test.location) ||
				!strings.Contains(err.Error(), "line "+strconv.Itoa(test.line)+":") {
				t.Fatalf("error=%v, want %q at line %d", err, test.location, test.line)
			}
			if got != nil {
				t.Fatalf("result escaped on error: %#v", got)
			}
		})
	}
}

func TestRunStepsRejectsUnsupportedExtractedNodes(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, input, reason string
	}{
		{"empty uses", `jobs: {ci: {steps: [{uses: ""}]}}`, "uses"},
		{"numeric uses", "jobs: {ci: {steps: [{uses: 42}]}}", "uses"},
		{"tagged run", "jobs: {ci: {steps: [{run: !custom make}]}}", "run"},
		{"environment key", "jobs: {ci: {env: {42: value}, steps: []}}", "env"},
		{"nested environment", "jobs: {ci: {env: {A: [one]}, steps: []}}", "env.A"},
		{"duplicate job", "jobs: {ci: {steps: []}, ci: {steps: []}}", "duplicate"},
		{"duplicate steps", "jobs: {ci: {steps: [], steps: []}}", "duplicate"},
		{"duplicate job environment", "jobs: {ci: {env: {A: one, A: two}, steps: []}}", "duplicate"},
		{"duplicate step environment", "jobs: {ci: {steps: [{run: make, env: {A: one, A: two}}]}}", "duplicate"},
		{"empty second document", "jobs: {}\n---\n", "document"},
		{"jobs alias", "x: &x {}\njobs: *x", "alias"},
		{"job key alias", "x: &x ci\njobs: {*x: {steps: []}}", "alias"},
		{"steps alias", "x: &x []\njobs: {ci: {steps: *x}}", "alias"},
		{"step alias", "x: &x {run: make}\njobs: {ci: {steps: [*x]}}", "alias"},
		{"run alias", "x: &x make\njobs: {ci: {steps: [{run: *x}]}}", "alias"},
		{"uses alias", "x: &x local/action\njobs: {ci: {steps: [{uses: *x}]}}", "alias"},
		{"name alias", "x: &x name\njobs: {ci: {steps: [{run: make, name: *x}]}}", "alias"},
		{"job environment alias", "x: &x {}\njobs: {ci: {env: *x, steps: []}}", "alias"},
		{"step environment alias", "x: &x {}\njobs: {ci: {steps: [{run: make, env: *x}]}}", "alias"},
		{"environment value alias", "x: &x value\njobs: {ci: {env: {A: *x}, steps: []}}", "alias"},
		{"environment key alias", "x: &x A\njobs: {ci: {env: {*x: value}, steps: []}}", "alias"},
		{"root merge", "<<: {jobs: {}}\njobs: {}", "merge"},
		{"jobs merge", "jobs: {<<: {ci: {steps: []}}}", "merge"},
		{"job merge", "jobs: {ci: {<<: {steps: []}}}", "merge"},
		{"step merge", "jobs: {ci: {steps: [{<<: {run: make}}]}}", "merge"},
		{"step environment merge", "jobs: {ci: {steps: [{run: make, env: {<<: {A: value}}}]}}", "merge"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := assets.RunSteps([]byte(test.input))
			if !errors.Is(err, assets.ErrRunSteps) ||
				!strings.Contains(err.Error(), test.reason) || !strings.Contains(err.Error(), "line ") {
				t.Fatalf("error=%v, want %q with line", err, test.reason)
			}
			if got != nil {
				t.Fatalf("result escaped on error: %#v", got)
			}
		})
	}
}

func TestRunStepsDiscardsEarlierResultsOnError(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, input string
	}{
		{"later run", "jobs: {ci: {steps: [{run: first}, {run: false}]}}"},
		{"later action", "jobs: {ci: {steps: [{run: first}, {uses: false}]}}"},
		{"later job", "jobs: {first: {steps: [{run: first}]}, last: {env: [], steps: []}}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := assets.RunSteps([]byte(test.input))
			if !errors.Is(err, assets.ErrRunSteps) || got != nil {
				t.Fatalf("steps=%#v, error=%v, want no partial result", got, err)
			}
		})
	}
}

func assertRunStepsRefusal(t *testing.T, got []assets.RunStep, err error, reason string) {
	t.Helper()
	if !errors.Is(err, assets.ErrRunSteps) || !strings.Contains(err.Error(), reason) {
		t.Fatalf("error=%v, want reason %q", err, reason)
	}
	if len(got) != 0 {
		t.Fatalf("partial result escaped on error: %#v", got)
	}
}
