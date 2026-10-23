package cicheck_test

import (
	"maps"
	"os"
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
		{"second default scope", "permissions: {contents: read}", "permissions: {contents: read, packages: write}"},
		{"job write scope", "  verify:\n", "  verify:\n    permissions:\n      contents: write\n"},
		{"job write-all", "  verify:\n", "  verify:\n    permissions: write-all\n"},
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

// cooldowns joins the two cooldown days of each ecosystem into one string,
// so a single map comparison catches a lane missing, added or shortened.
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

func TestVulnerabilityLaneFailsWithItsScanner(t *testing.T) {
	t.Parallel()
	root, err := document(kitBytes(t, ".github/workflows/vuln.yml"))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, step := range children(at(root, "jobs", "govulncheck", "steps")) {
		if value(at(step, "run")) == "make vuln" {
			count++
			if value(at(step, "continue-on-error")) == "true" {
				t.Fatal("the vulnerability command ignores failure")
			}
		}
	}
	if count != 1 {
		t.Fatalf("make vuln steps = %d, want 1", count)
	}
	requireText(t, "Makefile", "go run ./tools/vulngate -scan -waivers golaw.yml")
}

// TestKitTestTargetOutlastsTheSlowestPackage holds the kit's test target to
// a timeout above the slowest package under -race: go test's default of ten
// minutes per package panicked in internal/rung on a loaded machine, and a
// slower CI runner would fail make ci on the timeout, not on a test. The
// bound stays below the thirty minutes of the make ci job.
func TestKitTestTargetOutlastsTheSlowestPackage(t *testing.T) {
	t.Parallel()
	requireText(t, "Makefile", "test:\n\tgo test -race -timeout=25m -coverprofile=coverage.out -covermode=atomic ./...\n")
	requireText(t, ".github/workflows/ci.yml", "timeout-minutes: 30")
}
