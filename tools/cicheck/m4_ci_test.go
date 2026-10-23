package cicheck_test

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const m4Replay = "go run ./tools/acceptance -m4"

// rulesAgent is the agent whose prefix rules the kit ships, the CLI whose
// execpolicy check the M4 rows run and the lanes install.
func rulesAgent(t *testing.T) string {
	t.Helper()
	paths, err := filepath.Glob("../../agents/*/golaw.rules")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected one agent rules file, found %d", len(paths))
	}
	return filepath.Base(filepath.Dir(paths[0]))
}

// m4LaneFixture is a complete m4 job installing the CLI of agent.
func m4LaneFixture(agent string) string {
	return `
jobs:
  m4:
    runs-on: ${{ matrix.os }}
    timeout-minutes: 45
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    steps:
      - name: Install the pinned CLI
        run: npm install --global @vendor/` + agent + `@0.163.0
      - name: Replay M4 rows
        run: go run ./tools/acceptance -m4
`
}

func TestM4Lane(t *testing.T) {
	t.Parallel()
	agent := rulesAgent(t)
	fixture := m4LaneFixture(agent)
	install := "npm install --global @vendor/" + agent + "@0.163.0"
	for _, tt := range []struct{ name, body, want string }{
		{"repository", string(kitBytes(t, ".github/workflows/ci.yml")), ""},
		{"near complete", fixture, ""},
		{"corrupted YAML", "jobs: [", "M4 YAML cannot be decoded"},
		{"red absent", "jobs: {}\n", "missing M4 job"},
		{"missing Windows", strings.ReplaceAll(fixture, ", windows-latest", ""), "M4 requires three native systems"},
		{"wrong runner", strings.ReplaceAll(fixture, "runs-on: ${{ matrix.os }}", "runs-on: ubuntu-latest"), "M4 must use its native matrix"},
		{"short timeout", strings.ReplaceAll(fixture, "timeout-minutes: 45", "timeout-minutes: 10"), "M4 requires its bounded job"},
		{"fast cancellation", strings.ReplaceAll(fixture, "fail-fast: false", "fail-fast: true"), "M4 must finish all native systems"},
		{"wrong replay", strings.ReplaceAll(fixture, m4Replay, "go version"), "missing M4 replay"},
		{"conditional replay", strings.ReplaceAll(fixture, "- name: Replay M4 rows", "- if: false\n        name: Replay M4 rows"), "missing M4 replay"},
		{"ignored replay", strings.ReplaceAll(fixture, "- name: Replay M4 rows", "- continue-on-error: true\n        name: Replay M4 rows"), "missing M4 replay"},
		{"conditional job", strings.ReplaceAll(fixture, "  m4:\n", "  m4:\n    if: false\n"), "M4 job must be required"},
		{"unpinned CLI", strings.ReplaceAll(fixture, "@0.163.0", "@latest"), "M4 must install the pinned CLI before the replay"},
		{"no CLI", strings.ReplaceAll(fixture, install, "true"), "M4 must install the pinned CLI before the replay"},
		{"CLI after the replay", strings.Replace(strings.ReplaceAll(fixture, install, m4Replay), "- name: Replay M4 rows\n        run: "+m4Replay,
			"- name: Install\n        run: "+install, 1), "M4 must install the pinned CLI before the replay"},
	} {
		if got := m4LaneReason([]byte(tt.body), agent); got != tt.want {
			t.Fatalf("%s: reason=%q, want=%q", tt.name, got, tt.want)
		}
	}
}

func m4LaneReason(data []byte, agent string) string {
	root, err := document(data)
	if err != nil {
		return "M4 YAML cannot be decoded"
	}
	job := at(root, "jobs", "m4")
	switch {
	case job == nil:
		return "missing M4 job"
	case at(job, "if") != nil || at(job, "continue-on-error") != nil:
		return "M4 job must be required"
	case value(at(job, "runs-on")) != "${{ matrix.os }}":
		return "M4 must use its native matrix"
	case value(at(job, "timeout-minutes")) != "45":
		return "M4 requires its bounded job"
	case value(at(job, "strategy", "fail-fast")) != "false":
		return "M4 must finish all native systems"
	}
	var systems []string
	for _, item := range children(at(job, "strategy", "matrix", "os")) {
		systems = append(systems, value(item))
	}
	if !slices.Equal(systems, []string{"ubuntu-latest", "macos-latest", "windows-latest"}) {
		return "M4 requires three native systems"
	}
	return m4StepsReason(children(at(job, "steps")), agent)
}

// m4StepsReason requires the unconditional replay, preceded by the
// unconditional install of the CLI at an exact version.
func m4StepsReason(steps []*yaml.Node, agent string) string {
	replay := slices.IndexFunc(steps, func(step *yaml.Node) bool { return unconditional(step) && value(at(step, "run")) == m4Replay })
	if replay < 0 {
		return "missing M4 replay"
	}
	install := pinnedCLIStep(steps, agent)
	if install < 0 || install > replay {
		return "M4 must install the pinned CLI before the replay"
	}
	return ""
}

// unconditional reports whether a step always runs and always counts.
func unconditional(step *yaml.Node) bool {
	return at(step, "if") == nil && at(step, "continue-on-error") == nil
}

// pinnedCLIStep is the index of the unconditional step that installs the
// agent's CLI at an exact version, -1 when none does.
func pinnedCLIStep(steps []*yaml.Node, agent string) int {
	pinned := regexp.MustCompile(`^npm install --global @[a-z0-9-]+/` + regexp.QuoteMeta(agent) + `@\d+\.\d+\.\d+$`)
	return slices.IndexFunc(steps, func(step *yaml.Node) bool {
		return unconditional(step) && pinned.MatchString(value(at(step, "run")))
	})
}

func TestM4ReleaseInstallsTheSameCLI(t *testing.T) {
	t.Parallel()
	agent := rulesAgent(t)
	release := string(kitBytes(t, ".github/workflows/release.yml"))
	ci := string(kitBytes(t, ".github/workflows/ci.yml"))
	if got := m4ReleaseReason([]byte(ci), []byte(release), agent); got != "" {
		t.Fatal(got)
	}
	ciRoot, err := document([]byte(ci))
	if err != nil {
		t.Fatal(err)
	}
	steps := children(at(ciRoot, "jobs", "m4", "steps"))
	pinned := value(at(steps[pinnedCLIStep(steps, agent)], "run"))
	for name, body := range map[string]string{
		"red no CLI in the gate": strings.Replace(release, pinned, "true", 1),
		"red another version":    strings.Replace(release, pinned, strings.TrimSuffix(pinned, "0")+"1", 1),
		"red the CLI after the rows": strings.Replace(strings.Replace(release, "        run: "+pinned+"\n", "        run: true\n", 1),
			"run: go run ./tools/acceptance -release\n", "run: go run ./tools/acceptance -release\n      - name: Late\n        run: "+pinned+"\n", 1),
	} {
		if m4ReleaseReason([]byte(ci), []byte(body), agent) == "" {
			t.Errorf("%s passed", name)
		}
	}
}

// m4ReleaseReason requires the release gate to install, before its
// acceptance step, the CLI the m4 job installs, at the same version, so a
// required vendor row never skips there.
func m4ReleaseReason(ci, release []byte, agent string) string {
	ciRoot, err := document(ci)
	if err != nil {
		return "CI YAML cannot be decoded"
	}
	releaseRoot, err := document(release)
	if err != nil {
		return "release YAML cannot be decoded"
	}
	steps := children(at(ciRoot, "jobs", "m4", "steps"))
	gate := children(at(releaseRoot, "jobs", "gate", "steps"))
	lane, install := pinnedCLIStep(steps, agent), pinnedCLIStep(gate, agent)
	acceptance := slices.IndexFunc(gate, func(step *yaml.Node) bool { return value(at(step, "id")) == "acceptance" })
	switch {
	case lane < 0:
		return "the m4 job installs no pinned CLI"
	case install < 0 || acceptance < 0 || install > acceptance:
		return "the release gate must install the pinned CLI before its acceptance step"
	case value(at(gate[install], "run")) != value(at(steps[lane], "run")):
		return "the release gate and the m4 job pin two versions of the CLI"
	}
	return ""
}
