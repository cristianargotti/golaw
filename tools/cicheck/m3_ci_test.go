package cicheck_test

import (
	"slices"
	"strings"
	"testing"
)

const m3Replay = "go run ./tools/acceptance -m3"

const m3LaneFixture = `
jobs:
  m3:
    runs-on: ${{ matrix.os }}
    timeout-minutes: 90
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    steps:
      - name: Replay M3 rows
        run: go run ./tools/acceptance -m3
`

func TestM3Lane(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, body, want string }{
		{"repository", string(kitBytes(t, ".github/workflows/ci.yml")), ""},
		{"near complete", m3LaneFixture, ""},
		{"corrupted YAML", "jobs: [", "M3 YAML cannot be decoded"},
		{"red absent", "jobs: {}\n", "missing M3 job"},
		{"missing Windows", strings.ReplaceAll(m3LaneFixture, ", windows-latest", ""), "M3 requires three native systems"},
		{"wrong runner", strings.ReplaceAll(m3LaneFixture, "runs-on: ${{ matrix.os }}", "runs-on: ubuntu-latest"), "M3 must use its native matrix"},
		{"short timeout", strings.ReplaceAll(m3LaneFixture, "timeout-minutes: 90", "timeout-minutes: 10"), "M3 requires its bounded job"},
		{"fast cancellation", strings.ReplaceAll(m3LaneFixture, "fail-fast: false", "fail-fast: true"), "M3 must finish all native systems"},
		{"wrong replay", strings.ReplaceAll(m3LaneFixture, m3Replay, "go version"), "missing M3 replay"},
		{"conditional replay", strings.ReplaceAll(m3LaneFixture, "- name: Replay M3 rows", "- if: false\n        name: Replay M3 rows"), "missing M3 replay"},
		{"ignored replay", strings.ReplaceAll(m3LaneFixture, "- name: Replay M3 rows", "- continue-on-error: true\n        name: Replay M3 rows"), "missing M3 replay"},
		{"conditional job", strings.ReplaceAll(m3LaneFixture, "  m3:\n", "  m3:\n    if: false\n"), "M3 job must be required"},
	} {
		if got := m3LaneReason([]byte(tt.body)); got != tt.want {
			t.Fatalf("%s: reason=%q, want=%q", tt.name, got, tt.want)
		}
	}
}

func m3LaneReason(data []byte) string {
	root, err := document(data)
	if err != nil {
		return "M3 YAML cannot be decoded"
	}
	job := at(root, "jobs", "m3")
	switch {
	case job == nil:
		return "missing M3 job"
	case at(job, "if") != nil || at(job, "continue-on-error") != nil:
		return "M3 job must be required"
	case value(at(job, "runs-on")) != "${{ matrix.os }}":
		return "M3 must use its native matrix"
	case value(at(job, "timeout-minutes")) != "90":
		return "M3 requires its bounded job"
	case value(at(job, "strategy", "fail-fast")) != "false":
		return "M3 must finish all native systems"
	}
	var systems []string
	for _, item := range children(at(job, "strategy", "matrix", "os")) {
		systems = append(systems, value(item))
	}
	if !slices.Equal(systems, []string{"ubuntu-latest", "macos-latest", "windows-latest"}) {
		return "M3 requires three native systems"
	}
	for _, step := range children(at(job, "steps")) {
		if at(step, "if") == nil && at(step, "continue-on-error") == nil && value(at(step, "run")) == m3Replay {
			return ""
		}
	}
	return "missing M3 replay"
}
