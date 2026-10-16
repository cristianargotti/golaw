package cicheck_test

import (
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
)

// safeWorkflow is the near-miss lane of ci.safety as text.
func safeWorkflow(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("proof/testdata/ci.safety/near/.github/workflows/test.yml")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// ciMutation is one change that must make the safe lane unsafe.
type ciMutation struct {
	name string
	old  string
	next string
}

func ciMutations() []ciMutation {
	return []ciMutation{
		{"missing version", " # v5.0.0", ""},
		{"floating ref", "08c6903cd8c0fde910a37f88322edcfb5dd907a8", "v5"},
		{"write token", "contents: read", "contents: write"},
		{"persisted token", "persist-credentials: false", "persist-credentials: true"},
		{"quoted boolean", "persist-credentials: false", `persist-credentials: "false"`},
		{"missing restore", `echo "GOTOOLCHAIN=auto" >> "$GITHUB_ENV"`, "go version"},
		{"conditional restore", "- run:", "- if: false\n        run:"},
		{"ignored restore error", "- run:", "- continue-on-error: true\n        run:"},
	}
}

func TestCISafetyMutations(t *testing.T) {
	t.Parallel()
	source := safeWorkflow(t)
	if reason := inspectCI(".github/workflows/test.yml", []byte(source)); reason != "" {
		t.Fatal(reason)
	}
	for _, test := range ciMutations() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			changed := strings.Replace(source, test.old, test.next, 1)
			if changed == source {
				t.Fatal("mutation did not change the fixture")
			}
			if inspectCI(".github/workflows/test.yml", []byte(changed)) == "" {
				t.Fatal("unsafe CI accepted")
			}
		})
	}
}

func TestCISafetyRejectsExtraJobs(t *testing.T) {
	t.Parallel()
	source := safeWorkflow(t)
	secondSetup := source + `
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
`
	if inspectCI(".github/workflows/test.yml", []byte(secondSetup)) == "" {
		t.Fatal("second setup without restoration accepted")
	}
	secondJob := source + `
  other:
    steps:
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
      - run: go version
`
	if inspectCI(".github/workflows/test.yml", []byte(secondJob)) == "" {
		t.Fatal("unsafe second job accepted")
	}
}

// cooldowns maps each ecosystem of a Dependabot file to its default and
// major cooldown days.
func cooldowns(t *testing.T, data []byte) map[string]string {
	t.Helper()
	root, err := document(data)
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]string)
	for _, update := range children(at(root, "updates")) {
		out[value(at(update, "package-ecosystem"))] = value(at(update, "cooldown", "default-days")) +
			"/" + value(at(update, "cooldown", "semver-major-days"))
	}
	return out
}

func TestSupplyChainLanes(t *testing.T) {
	t.Parallel()
	want := map[string]string{"gomod": "7/30", "github-actions": "7/"}
	data := kitBytes(t, ".github/dependabot.yml")
	if got := cooldowns(t, data); !maps.Equal(got, want) {
		t.Fatalf("Dependabot cooldowns = %v, want %v", got, want)
	}
	shortened := strings.Replace(string(data), "default-days: 7", "default-days: 1", 1)
	if maps.Equal(cooldowns(t, []byte(shortened)), want) {
		t.Fatal("a shortened cooldown accepted")
	}
	requireText(t, "Makefile", "go run ./tools/depsage -days 7", "ci: mod deps-age ")
}

// A step without a shell runs as bash -e, which takes the exit code of the
// last command of a pipe, so a scanner that fails would leave vulngate to
// read an empty stream; only shell: bash adds pipefail.
func TestVulnerabilityLaneFailsWithItsScanner(t *testing.T) {
	t.Parallel()
	root, err := document(kitBytes(t, ".github/workflows/vuln.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var shells []string
	for _, step := range children(at(root, "jobs", "govulncheck", "steps")) {
		if strings.Contains(value(at(step, "run")), "| go run ./tools/vulngate ") {
			shells = append(shells, value(at(step, "shell")))
		}
	}
	if !slices.Equal(shells, []string{"bash"}) {
		t.Fatalf("shells of the piped scan = %q, want [bash]", shells)
	}
}
