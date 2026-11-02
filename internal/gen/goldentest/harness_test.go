package goldentest_test

import (
	"context"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/goldentest"
	"github.com/cristianargotti/golaw/internal/gen/kinds/support"
	"github.com/cristianargotti/golaw/internal/gen/kinds/wiring"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
)

// drifting renders one more declaration on every regeneration than golaw
// new wrote, the shape of a template that is not deterministic.
type drifting struct{ greeter }

func (d drifting) Regenerate(ctx context.Context, in genkind.Input, entry manifest.Entry) (genkind.Output, error) {
	regenerated := greeter{extra: "\n// Drifted is rendered by a regeneration alone.\nconst Drifted = 1\n"}
	return regenerated.Regenerate(ctx, in, entry)
}

// The red extras the harness's other steps reject: a body the compiler
// refuses, a declaration the formatter rewrites, a function past the size
// law of the scan and a function no test runs.
const (
	brokenBuild = `
// Broken answers a string where an int is declared.
func Broken() int {
	return "broken"
}
`
	unformatted = `
// Spaced is a declaration the formatter rewrites.
var Spaced   = 1
`
	// pastTheFloor uses new with an expression, which Go 1.26 allows and
	// the go 1.24 floor of an adopter (A9) refuses.
	pastTheFloor = `
// Floor points at a value the way only Go 1.26 allows.
var Floor = new(1)
`
)

// untested is a regenerable function no generated test runs, of enough
// statements to sink the new code under the floor whatever the size of the
// support package the slice also writes, which its own tests cover.
func untested() string {
	return "\n// Untested holds statements no generated test runs.\nfunc Untested(values []int) int {\n\ttotal := 0\n" +
		strings.Repeat("\ttotal += len(values)\n", 2000) + "\treturn total\n}\n"
}

// longFunction is a regenerable function past the forty line size law.
func longFunction() string {
	return "\n// Long counts to forty five.\nfunc Long() int {\n\tcount := 0\n" + strings.Repeat("\tcount++\n", 45) + "\treturn count\n}\n"
}

func TestExecFailsEachRedSliceAtTheOtherSteps(t *testing.T) {
	t.Parallel()
	drift := slice("", bothFills()...)
	drift.Kinds = []genkind.Kind{support.Kind(), wiring.Kind(), drifting{}}
	unparsed := []goldentest.Fill{{ID: "greeter.greet", Body: "return ("}, bothFills()[1]}
	docOnlyGenerated := slice("", bothFills()...)
	docOnlyGenerated.Kinds = []genkind.Kind{support.Kind(), wiring.Kind(), greeter{docInRegenerable: true}}
	ambiguous := []goldentest.Fill{{ID: "greeter.greet", Body: "return template.HTMLEscapeString(name), ctx.Err()"}, bothFills()[1]}
	tests := map[string]struct {
		slice goldentest.Slice
		steps []goldentest.Step
		want  string // a fragment of one failure
	}{
		"red: a regenerable file that does not compile":        {slice(brokenBuild, bothFills()...), []goldentest.Step{goldentest.StepBuild}, "build: go build ./...: exit 1"},
		"red: a regenerable file the formatter rewrites":       {slice(unformatted, bothFills()...), []goldentest.Step{goldentest.StepFormat}, "differs from golangci-lint fmt"},
		"red: a regenerable function past the size law":        {slice(longFunction(), bothFills()...), []goldentest.Step{goldentest.StepScan}, "scan: internal/adapters/out/greet/zz_generated_greeter.go:"},
		"red: new code under the floor with the fills":         {slice(untested(), bothFills()...), []goldentest.Step{goldentest.StepCoverage}, "percent, at least 80: internal/adapters/out/greet/zz_generated_greeter.go ["},
		"red: a template whose regeneration drifts":            {drift, []goldentest.Step{goldentest.StepCheck}, "check: drift internal/adapters/out/greet/zz_generated_greeter.go "},
		"red: a fill whose body does not parse":                {slice("", unparsed...), []goldentest.Step{goldentest.StepCheck}, "fill greeter.greet: "},
		"red: a fill whose import is not certain":              {slice("", ambiguous...), []goldentest.Step{goldentest.StepCheck}, "fill greeter.greet: imports left as they were: "},
		"red: a package comment in the regenerable file alone": {docOnlyGenerated, []goldentest.Step{goldentest.StepLint}, "lint: as the adopter runs it: exit 1: "},
		"red: a regenerable file past the adopter's go floor":  {slice(pastTheFloor, bothFills()...), []goldentest.Step{goldentest.StepFloor}, "floor: go vet ./... at go 1.24: exit 1: "},
		"red: a package under the floor of the adopter's coverage-check": {
			slice(untested(), bothFills()...),
			[]goldentest.Step{goldentest.StepCoverage},
			"below the floor of the adopter's coverage-check: example.com/slice/internal/adapters/out/greet ",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			report, failures := goldentest.Exec(t, tc.slice, tc.steps...)
			for _, step := range append(tc.steps, goldentest.StepFormat) {
				if reason, skipped := report.Skipped[step]; skipped {
					t.Skip(reason)
				}
			}
			if joined := strings.Join(failures, "\n"); !strings.Contains(joined, tc.want) {
				t.Fatalf("failures = %s, want one holding %q", joined, tc.want)
			}
		})
	}
}

// Red row: the surface step judges under the strict set init writes into a
// new policy, never under the parity set of the slice's golaw.yml, which
// has no rule for the YAML a slice writes and so judged nothing. With no
// surface tool installed the step still skips, naming the absent tool.
func TestSurfaceStepJudgesUnderTheStrictSet(t *testing.T) {
	t.Parallel()
	report, failures := goldentest.Exec(t, slice("", bothFills()...), goldentest.StepSurface)
	if reason := report.Skipped[goldentest.StepSurface]; strings.Contains(reason, "parity set") || len(failures) != 0 {
		t.Errorf("surface skipped with %q, failures %q: want the strict set judging, or the absent tool named", reason, failures)
	}
}
