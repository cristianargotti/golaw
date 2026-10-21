package assets_test

import (
	"maps"
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
)

func TestRunStepsPreservesOrderAndEnvironment(t *testing.T) {
	t.Parallel()
	const input = `name: CI
env: {OUTSIDE: root}
jobs:
  z:
    env: {GOFLAGS: "-mod=readonly", VALUE: job}
    steps:
      - uses: actions/checkout@08c6903cd8c0fde910a37f88322edcfb5dd907a8
      - name: first
        run: |
          go version
          make ci
        env: {VALUE: step, EMPTY: ""}
      - run: git diff --exit-code
  a:
    steps:
      - name: final
        run: make lint
`
	want := []assets.RunStep{
		{
			Job: "z", Name: "first", Run: "go version\nmake ci\n",
			Env: map[string]string{
				"GOFLAGS": "-mod=readonly", "VALUE": "step", "EMPTY": "",
			},
		},
		{
			Job: "z", Run: "git diff --exit-code",
			Env: map[string]string{"GOFLAGS": "-mod=readonly", "VALUE": "job"},
		},
		{Job: "a", Name: "final", Run: "make lint", Env: map[string]string{}},
	}
	got, err := assets.RunSteps([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("steps=%#v, want %#v", got, want)
	}
	got[0].Env["VALUE"] = "changed"
	if got[1].Env["VALUE"] != "job" {
		t.Fatal("steps share their environment map")
	}
	again, err := assets.RunSteps([]byte(input))
	if err != nil || !maps.Equal(again[0].Env, want[0].Env) {
		t.Fatalf("reader retained caller mutation: %#v, %v", again, err)
	}
}

func TestRunStepsKeepsExpressionsLiteral(t *testing.T) {
	t.Parallel()
	const input = `jobs:
  ci:
    steps:
      - run: make ci
        env:
          GOLAW_ENROLLED: "${{ vars.GOLAW_ENROLLED }}"
`
	got, err := assets.RunSteps([]byte(input))
	if err != nil || len(got) != 1 {
		t.Fatalf("steps=%#v, error=%v", got, err)
	}
	if got[0].Env["GOLAW_ENROLLED"] != ciEnrollmentExpression {
		t.Fatalf("expression was interpreted: %#v", got[0].Env)
	}
}

func TestRunStepsReadsEveryRenderedCombination(t *testing.T) {
	t.Parallel()
	for _, caps := range ciCapabilitySets() {
		context := assetContext()
		context.Capabilities = caps
		file := renderedAsset(t, ciAssetPath, context)
		steps, err := assets.RunSteps(file.Data)
		if err != nil {
			t.Fatal(err)
		}
		if len(steps) == 0 {
			t.Fatalf("no run steps for %v", caps)
		}
		seen := make(map[string]bool)
		for _, step := range steps {
			if step.Job == "" || step.Run == "" {
				t.Fatalf("incomplete step: %+v", step)
			}
			seen[step.Job] = true
		}
		if !seen["ci"] {
			t.Fatalf("ci job was omitted for %v", caps)
		}
	}
}

func TestRunStepsReturnsAllocatedEmptyResults(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		"jobs: {}\n",
		"jobs: {ci: {steps: []}}\n",
		"jobs: {ci: {steps: [{uses: local/action}]}}\n",
	} {
		got, err := assets.RunSteps([]byte(input))
		if err != nil || got == nil || len(got) != 0 {
			t.Fatalf("steps=%#v, error=%v, want allocated empty result", got, err)
		}
	}
}

func TestRunStepsPreservesCommandContent(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, command, want string
	}{
		{"spaces", `"   "`, "   "},
		{"escaped whitespace", `" \tprintf 'ok' \n"`, " \tprintf 'ok' \n"},
		{"multiline", "|-\n          first\n\n          second  \n", "first\n\nsecond  "},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := "jobs:\n  ci:\n    steps:\n      - run: " + test.command + "\n"
			got, err := assets.RunSteps([]byte(input))
			if err != nil || len(got) != 1 {
				t.Fatalf("steps=%#v, error=%v", got, err)
			}
			if got[0].Run != test.want {
				t.Fatalf("command=%q, want %q", got[0].Run, test.want)
			}
		})
	}
}

func TestRunStepsIgnoresUnextractedMetadata(t *testing.T) {
	t.Parallel()
	const input = `x: &metadata {<<: {X: value}, nested: [false]}
name: *metadata
env: *metadata
jobs:
  ci:
    name: *metadata
    strategy: {matrix: *metadata}
    if: *metadata
    steps:
      - uses: local/action
        name: *metadata
        env: *metadata
        with: *metadata
      - run: &command make ci
        name: &name ""
        env: {"<<": literal, A: &value ""}
        if: *metadata
        working-directory: *metadata
# Trailing comments do not create another document.
`
	want := []assets.RunStep{
		{Job: "ci", Run: "make ci", Env: map[string]string{"<<": "literal", "A": ""}},
	}
	got, err := assets.RunSteps([]byte(input))
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("steps=%#v, error=%v, want %#v", got, err, want)
	}
}
