package goldentest_test

import (
	"os"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/goldentest"
	"github.com/cristianargotti/golaw/internal/gen/kinds/support"
	"github.com/cristianargotti/golaw/internal/gen/kinds/wiring"
	"github.com/cristianargotti/golaw/internal/gentest"
)

const greetFill = `return g.prefix + " " + name, ctx.Err()`

// slice is two instances of the fake kind over the growth fixture.
func slice(extra string, fills ...goldentest.Fill) goldentest.Slice {
	return goldentest.Slice{
		Name: "greeter", Module: "example.com/slice", Base: gentest.Contracts(),
		Kinds: []genkind.Kind{support.Kind(), wiring.Kind(), greeter{extra: extra}},
		Commands: []goldentest.Command{
			{Kind: genkind.KindAdapterHTTP, Request: genkind.Request{Name: genkind.Name{Area: "greet", Type: "Greeter"}}},
			{Kind: genkind.KindAdapterHTTP, Request: genkind.Request{Name: genkind.Name{Area: "welcome", Type: "Welcomer"}}},
		},
		Fills: fills,
	}
}

func bothFills() []goldentest.Fill {
	return []goldentest.Fill{{ID: "greeter.greet", Body: greetFill}, {ID: "welcomer.greet", Body: greetFill}}
}

// The near slice runs every step with the real tools, so it runs alone:
// beside the parallel red slices on a loaded machine a step's tool met
// its own timeout once, and a required row must not depend on the load.
func TestGoldenHarnessNearSliceIsGreenByConstruction(t *testing.T) { //nolint:paralleltest // see above
	report := goldentest.Run(t, slice("", bothFills()...))
	if report.Coverage < 80 || !report.OpenCovered || report.Duplication >= 1.5 {
		t.Errorf("coverage %v, open covered %v, duplication %v", report.Coverage, report.OpenCovered, report.Duplication)
	}
	for _, want := range []string{
		"internal/adapters/out/greet/zz_generated_greeter.go", "internal/adapters/out/greet/greeter.go",
		"internal/adapters/out/welcome/zz_generated_welcomer_test.go", "internal/gensupport/zz_generated_hole.go",
	} {
		if len(report.Files[want]) == 0 {
			t.Errorf("the commands wrote no %s", want)
		}
	}
	if info, err := os.Stat(report.Dir); err != nil || !info.IsDir() {
		t.Errorf("Dir = %q: %v", report.Dir, err)
	}
	for step, reason := range report.Skipped {
		if reason == "" {
			t.Errorf("step %s skipped without its reason", step)
		}
	}
	if strings.Contains(string(report.Files["internal/adapters/out/greet/greeter.go"]), greetFill) {
		t.Error("Files holds the filled bytes; it must hold what the commands wrote")
	}
}

func TestExecFailsEachRedSliceAtItsStep(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		slice goldentest.Slice
		steps []goldentest.Step
		want  string // a fragment of one failure; empty for none
	}{
		"red: a regenerable function with eight parameters": {
			slice(eightParameters, bothFills()...), []goldentest.Step{goldentest.StepLaws}, "S107-parameters: 8 parameters",
		},
		"red: a hole left without its fill": {
			slice("", bothFills()[0]), []goldentest.Step{goldentest.StepCheck}, "check: hole open internal/adapters/out/welcome/welcomer.go:",
		},
		"red: a fill that names no hole": {
			slice("", append(bothFills(), goldentest.Fill{ID: "nobody.none", Body: "return"})...),
			[]goldentest.Step{goldentest.StepCheck},
			"fill nobody.none: names no open hole",
		},
		"red: a regenerable line no generated test reaches": {
			slice(uncovered, bothFills()...), []goldentest.Step{goldentest.StepCoverage}, "regenerable line not covered with the holes open",
		},
		"red: one body copied into both instances": {
			slice(copied, bothFills()...), []goldentest.Step{goldentest.StepDup}, "dup: ",
		},
		"near: the thin instances pass the in process steps": {
			slice("", bothFills()...), []goldentest.Step{goldentest.StepLaws, goldentest.StepDup, goldentest.StepCheck}, "",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			report, failures := goldentest.Exec(t, tc.slice, tc.steps...)
			joined := strings.Join(failures, "\n")
			if tc.want == "" && len(failures) != 0 {
				t.Fatalf("failures = %s, want none", joined)
			}
			if tc.want != "" && !strings.Contains(joined, tc.want) {
				t.Fatalf("failures = %s, want one holding %q", joined, tc.want)
			}
			if report.Dir == "" || len(report.Files) == 0 {
				t.Errorf("report = %+v, want the module and the written files", report)
			}
		})
	}
}

func TestExecRefusesAnIncompleteSlice(t *testing.T) {
	t.Parallel()
	noPolicy := gentest.Contracts()
	delete(noPolicy, "golaw.yml")
	unknown := slice("", bothFills()...)
	unknown.Commands = []goldentest.Command{{Kind: genkind.KindHandler, Request: genkind.Request{Name: genkind.Name{Area: "cockpitapi", Type: "Handler"}}}}
	tests := map[string]struct {
		slice goldentest.Slice
		steps []goldentest.Step
		want  string
	}{
		"red: no module":           {goldentest.Slice{Name: "x", Base: gentest.Contracts(), Kinds: slice("").Kinds, Commands: slice("").Commands}, nil, "slice: invalid golden slice: no module"},
		"red: no golaw.yml":        {goldentest.Slice{Name: "x", Module: "example.com/slice", Base: noPolicy, Kinds: slice("").Kinds, Commands: slice("").Commands}, nil, "slice: invalid golden slice: the base holds no golaw.yml"},
		"red: no command":          {goldentest.Slice{Name: "x", Module: "example.com/slice", Base: gentest.Contracts(), Kinds: slice("").Kinds}, nil, "slice: invalid golden slice: no command"},
		"red: an unknown step":     {slice("", bothFills()...), []goldentest.Step{"nosuch"}, "slice: invalid golden slice: unknown step nosuch"},
		"red: a kind not in Kinds": {unknown, []goldentest.Step{goldentest.StepLaws}, "command handler cockpitapi.Handler: "},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, failures := goldentest.Exec(t, tc.slice, tc.steps...)
			if joined := strings.Join(failures, "\n"); !strings.Contains(joined, tc.want) {
				t.Fatalf("failures = %s, want one holding %q", joined, tc.want)
			}
		})
	}
	if goldentest.ErrSlice.Error() != "invalid golden slice" {
		t.Errorf("ErrSlice = %v", goldentest.ErrSlice)
	}
}

func TestTheDSLSumsAreTheKitsOwn(t *testing.T) {
	t.Parallel()
	sums, err := os.ReadFile("../../../go.sum")
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(goldentest.DSLSums), "\n") {
		if !strings.Contains(string(sums), line+"\n") {
			t.Errorf("the kit's go.sum lacks %q", line)
		}
	}
	template, err := os.ReadFile("../../../templates/go.mod.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(template), "github.com/quasilyte/go-ruleguard/dsl "+goldentest.DSLVersion) {
		t.Errorf("templates/go.mod.tmpl requires another dsl than %s", goldentest.DSLVersion)
	}
}
