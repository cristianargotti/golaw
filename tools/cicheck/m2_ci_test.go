package cicheck_test

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const m2Replay = "go run ./tools/acceptance -m2"

const m2Measure = "go test -race -timeout=45m -run '^TestM2Measurements$' -count=1 -v ./tools/acceptance/"

const m2LaneFixture = `
jobs:
  m2:
    runs-on: ${{ matrix.os }}
    timeout-minutes: 60
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    steps:
      - name: Replay M2 rows
        run: go run ./tools/acceptance -m2
      - name: Measure native rungs
        env:
          GOLAW_M2_MEASURE: '1'
        run: go test -race -timeout=45m -run '^TestM2Measurements$' -count=1 -v ./tools/acceptance/
`

func TestM2NativeLane(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want string
	}{
		{"repository", string(kitBytes(t, ".github/workflows/ci.yml")), ""},
		{"near complete", m2LaneFixture, ""},
		{"corrupted YAML", "jobs: [", "M2 YAML cannot be decoded"},
		{"red absent", "jobs: {}\n", "missing M2 job"},
		{"missing Windows", strings.ReplaceAll(m2LaneFixture,
			", windows-latest", ""), "M2 requires three native systems"},
		{
			"wrong runner", strings.ReplaceAll(m2LaneFixture,
				"runs-on: ${{ matrix.os }}", "runs-on: ubuntu-latest"),
			"M2 must use its native matrix",
		},
		{
			"short timeout", strings.ReplaceAll(m2LaneFixture,
				"timeout-minutes: 60", "timeout-minutes: 10"),
			"M2 requires its bounded native job",
		},
		{
			"fast cancellation", strings.ReplaceAll(m2LaneFixture,
				"fail-fast: false", "fail-fast: true"),
			"M2 must finish all native systems",
		},
		{"wrong replay", strings.ReplaceAll(m2LaneFixture,
			m2Replay, "go version"), "missing M2 replay"},
		{"missing measurements", strings.ReplaceAll(m2LaneFixture,
			m2Measure, "go version"), "missing M2 measurements"},
		{
			"disabled measurements", strings.ReplaceAll(m2LaneFixture,
				"GOLAW_M2_MEASURE: '1'", "GOLAW_M2_MEASURE: '0'"),
			"missing M2 measurements",
		},
		{
			"conditional replay", strings.ReplaceAll(m2LaneFixture,
				"- name: Replay M2 rows",
				"- if: false\n        name: Replay M2 rows"),
			"missing M2 replay",
		},
		{
			"ignored replay", strings.ReplaceAll(m2LaneFixture,
				"- name: Replay M2 rows",
				"- continue-on-error: true\n        name: Replay M2 rows"),
			"missing M2 replay",
		},
		{
			"conditional measurement", strings.ReplaceAll(m2LaneFixture,
				"- name: Measure native rungs",
				"- if: false\n        name: Measure native rungs"),
			"missing M2 measurements",
		},
		{
			"ignored measurement", strings.ReplaceAll(m2LaneFixture,
				"- name: Measure native rungs",
				"- continue-on-error: true\n        name: Measure native rungs"),
			"missing M2 measurements",
		},
		{
			"conditional job", strings.ReplaceAll(m2LaneFixture,
				"  m2:\n", "  m2:\n    if: false\n"),
			"M2 job must be required",
		},
		{
			"ignored job", strings.ReplaceAll(m2LaneFixture,
				"  m2:\n", "  m2:\n    continue-on-error: true\n"),
			"M2 job must be required",
		},
		{
			"live database in the required lane", strings.ReplaceAll(m2LaneFixture,
				"GOLAW_M2_MEASURE: '1'", "GOLAW_M2_MEASURE: '1'\n          GOLAW_M2_LIVE_DB: '1'"),
			"M2 required measurements use the fixture database",
		},
	}
	for _, tt := range tests {
		if got := m2LaneReason([]byte(tt.body)); got != tt.want {
			t.Fatalf("%s: reason=%q, want=%q", tt.name, got, tt.want)
		}
	}
}

func m2LaneReason(data []byte) string {
	root, err := document(data)
	if err != nil {
		return "M2 YAML cannot be decoded"
	}
	job := at(root, "jobs", "m2")
	if job == nil {
		return "missing M2 job"
	}
	if at(job, "if") != nil || at(job, "continue-on-error") != nil {
		return "M2 job must be required"
	}
	if value(at(job, "runs-on")) != "${{ matrix.os }}" {
		return "M2 must use its native matrix"
	}
	if value(at(job, "timeout-minutes")) != "60" {
		return "M2 requires its bounded native job"
	}
	if value(at(job, "strategy", "fail-fast")) != "false" {
		return "M2 must finish all native systems"
	}
	var systems []string
	for _, item := range children(at(job, "strategy", "matrix", "os")) {
		systems = append(systems, value(item))
	}
	if !slices.Equal(systems,
		[]string{"ubuntu-latest", "macos-latest", "windows-latest"}) {
		return "M2 requires three native systems"
	}
	return m2StepsReason(children(at(job, "steps")))
}

// m2StepsReason requires the unconditional replay and the unconditional
// measurement of the required job, timed against the fixture database.
func m2StepsReason(steps []*yaml.Node) string {
	replay, measure, live := false, false, false
	for _, step := range steps {
		if at(step, "if") != nil || at(step, "continue-on-error") != nil {
			continue
		}
		switch value(at(step, "run")) {
		case m2Replay:
			replay = true
		case m2Measure:
			measure = value(at(step, "env", "GOLAW_M2_MEASURE")) == "1"
			live = at(step, "env", "GOLAW_M2_LIVE_DB") != nil
		}
	}
	switch {
	case !replay:
		return "missing M2 replay"
	case !measure:
		return "missing M2 measurements"
	case live:
		return "M2 required measurements use the fixture database"
	}
	return ""
}

const m2LiveFixture = `
on:
  schedule:
    - cron: '17 6 * * 1'
  workflow_dispatch:
jobs:
  m2-live:
    runs-on: ubuntu-latest
    timeout-minutes: 60
    steps:
      - name: Time the rungs against the live database
        env:
          GOLAW_M2_MEASURE: '1'
          GOLAW_M2_LIVE_DB: '1'
        run: go test -race -timeout=45m -run '^TestM2Measurements$' -count=1 -v ./tools/acceptance/
`

func TestM2LiveTimingsStayScheduled(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want string
	}{
		{"repository", string(kitBytes(t, ".github/workflows/vuln.yml")), ""},
		{"near scheduled", m2LiveFixture, ""},
		{"corrupted YAML", "jobs: [", "live YAML cannot be decoded"},
		{"red absent", "on:\n  workflow_dispatch:\njobs: {}\n", "missing M2 live timings"},
		{"pull request", strings.ReplaceAll(m2LiveFixture,
			"  workflow_dispatch:\n", "  workflow_dispatch:\n  pull_request:\n"), "live timings must stay scheduled"},
		{"push", strings.ReplaceAll(m2LiveFixture,
			"  workflow_dispatch:\n", "  workflow_dispatch:\n  push:\n"), "live timings must stay scheduled"},
		{"fixture database", strings.ReplaceAll(m2LiveFixture,
			"          GOLAW_M2_LIVE_DB: '1'\n", ""), "missing M2 live timings"},
	}
	for _, tt := range tests {
		if got := m2LiveReason([]byte(tt.body)); got != tt.want {
			t.Fatalf("%s: reason=%q, want=%q", tt.name, got, tt.want)
		}
	}
}

// m2LiveReason requires the live-database timings in a lane that only a
// schedule or a person starts, never a pull request or a push.
func m2LiveReason(data []byte) string {
	root, err := document(data)
	if err != nil {
		return "live YAML cannot be decoded"
	}
	if at(root, "on", "pull_request") != nil || at(root, "on", "pull_request_target") != nil ||
		at(root, "on", "push") != nil {
		return "live timings must stay scheduled"
	}
	for _, step := range children(at(root, "jobs", "m2-live", "steps")) {
		if value(at(step, "run")) == m2Measure &&
			value(at(step, "env", "GOLAW_M2_MEASURE")) == "1" &&
			value(at(step, "env", "GOLAW_M2_LIVE_DB")) == "1" {
			return ""
		}
	}
	return "missing M2 live timings"
}
