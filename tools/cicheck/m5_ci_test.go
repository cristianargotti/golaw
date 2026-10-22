package cicheck_test

import (
	"slices"
	"strings"
	"testing"
)

const m5Replay = "go run ./tools/acceptance -m5"

const m5LaneFixture = `
jobs:
  m5:
    runs-on: ${{ matrix.os }}
    timeout-minutes: 90
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    steps:
      - name: Replay M5 rows
        run: go run ./tools/acceptance -m5
`

func TestM5Lane(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, body, want string }{
		{"repository", string(kitBytes(t, ".github/workflows/ci.yml")), ""},
		{"near complete", m5LaneFixture, ""},
		{"corrupted YAML", "jobs: [", "M5 YAML cannot be decoded"},
		{"red absent", "jobs: {}\n", "missing M5 job"},
		{"missing Windows", strings.ReplaceAll(m5LaneFixture, ", windows-latest", ""), "M5 requires three native systems"},
		{"wrong runner", strings.ReplaceAll(m5LaneFixture, "runs-on: ${{ matrix.os }}", "runs-on: ubuntu-latest"), "M5 must use its native matrix"},
		{"short timeout", strings.ReplaceAll(m5LaneFixture, "timeout-minutes: 90", "timeout-minutes: 10"), "M5 requires its bounded job"},
		{"fast cancellation", strings.ReplaceAll(m5LaneFixture, "fail-fast: false", "fail-fast: true"), "M5 must finish all native systems"},
		{"wrong replay", strings.ReplaceAll(m5LaneFixture, m5Replay, "go version"), "missing M5 replay"},
		{"conditional replay", strings.ReplaceAll(m5LaneFixture, "- name: Replay M5 rows", "- if: false\n        name: Replay M5 rows"), "missing M5 replay"},
		{"ignored replay", strings.ReplaceAll(m5LaneFixture, "- name: Replay M5 rows", "- continue-on-error: true\n        name: Replay M5 rows"), "missing M5 replay"},
		{"conditional job", strings.ReplaceAll(m5LaneFixture, "  m5:\n", "  m5:\n    if: false\n"), "M5 job must be required"},
	} {
		if got := m5LaneReason([]byte(tt.body)); got != tt.want {
			t.Fatalf("%s: reason=%q, want=%q", tt.name, got, tt.want)
		}
	}
}

func m5LaneReason(data []byte) string {
	root, err := document(data)
	if err != nil {
		return "M5 YAML cannot be decoded"
	}
	job := at(root, "jobs", "m5")
	switch {
	case job == nil:
		return "missing M5 job"
	case at(job, "if") != nil || at(job, "continue-on-error") != nil:
		return "M5 job must be required"
	case value(at(job, "runs-on")) != "${{ matrix.os }}":
		return "M5 must use its native matrix"
	case value(at(job, "timeout-minutes")) != "90":
		return "M5 requires its bounded job"
	case value(at(job, "strategy", "fail-fast")) != "false":
		return "M5 must finish all native systems"
	}
	var systems []string
	for _, item := range children(at(job, "strategy", "matrix", "os")) {
		systems = append(systems, value(item))
	}
	if !slices.Equal(systems, []string{"ubuntu-latest", "macos-latest", "windows-latest"}) {
		return "M5 requires three native systems"
	}
	for _, step := range children(at(job, "steps")) {
		if at(step, "if") == nil && at(step, "continue-on-error") == nil && value(at(step, "run")) == m5Replay {
			return ""
		}
	}
	return "missing M5 replay"
}

// TestM5JobFollowsTheM4Job keeps the acceptance jobs in milestone order.
// The m4 job lands with the M4 acceptance lane, which this tree does not
// hold yet, so until it does the m5 job follows the m3 job, the last
// acceptance job before it; once m4 exists, m5 must follow m4.
func TestM5JobFollowsTheM4Job(t *testing.T) {
	t.Parallel()
	root, err := document(kitBytes(t, ".github/workflows/ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	content := children(at(root, "jobs"))
	jobs := make([]string, 0, len(content)/2)
	for i := 0; i+1 < len(content); i += 2 {
		jobs = append(jobs, content[i].Value)
	}
	m4, m5 := slices.Index(jobs, "m4"), slices.Index(jobs, "m5")
	if m4 < 0 {
		m4 = slices.Index(jobs, "m3")
	}
	if m4 < 0 || m5 != m4+1 {
		t.Fatalf("jobs = %q: m5 must follow m4", jobs)
	}
}
