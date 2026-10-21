package policyfiles

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestActionsReaderKeepsOrderedEvidence(t *testing.T) {
	t.Parallel()
	const first = ".github/workflows/a.yml"
	const second = ".github/workflows/b.yaml"
	files := map[string]string{
		first: "on: [push, workflow_dispatch]\njobs:\n  check:\n    steps:\n" +
			"      - uses: owner/action@0123456789012345678901234567890123456789\n" +
			"      - run: make vuln\n        env:\n          GOTOOLCHAIN: local\n",
		second:                                 "on:\n  schedule:\n    - cron: '0 0 * * *'\njobs:\n  audit:\n    steps:\n      - run: make licenses\n",
		".github/workflows/nested/ignored.yml": "not: [valid",
	}
	got, err := readActions(makeInput(files))
	if err != nil || len(got) != 2 {
		t.Fatalf("files = %+v, %v", got, err)
	}
	if got[0].path != first || got[1].path != second {
		t.Fatalf("file order = %+v", got)
	}
	if !slices.Equal(got[0].triggers, []string{"push", "workflow_dispatch"}) {
		t.Fatalf("triggers = %v", got[0].triggers)
	}
	job := got[0].jobs[0]
	if job.id != "check" || job.line == 0 || len(job.steps) != 2 {
		t.Fatalf("job = %+v", job)
	}
	step := job.steps[1]
	if step.run != "make vuln" || step.env["GOTOOLCHAIN"] != "local" || step.line == 0 {
		t.Fatalf("step = %+v", step)
	}
}

func TestActionsReaderLimitsAndNearMisses(t *testing.T) {
	t.Parallel()
	const file = ".github/workflows/check.yml"
	for _, tc := range []struct {
		name, body string
		limited    bool
	}{
		{"near scalar event", "on: push\njobs: {}\n", false},
		{"near empty tree", "", false},
		{"red dynamic events", "on: '${{ inputs.events }}'\njobs: {}\n", true},
		{"red run sequence", "on: push\njobs:\n  check:\n    steps:\n      - run: [make, ci]\n", true},
		{"near reusable job", "on: schedule\njobs:\n  check:\n    uses: owner/repo/file@v1\n", false},
		{"near shell text retained", "on: push\njobs:\n  check:\n    steps:\n      - run: |\n          echo ready\n          make ci\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := readActions(makeInput(map[string]string{file: tc.body}))
			if tc.limited {
				if !errors.Is(err, model.ErrOutsideSubset) {
					t.Fatalf("limit = %v", err)
				}
				return
			}
			if err != nil || len(got) != 1 {
				t.Fatalf("read = %+v, %v", got, err)
			}
			if tc.name == "near reusable job" {
				requireActionsReusableJob(t, got[0].jobs)
			}
		})
	}
}

func requireActionsReusableJob(t *testing.T, jobs []actionsJob) {
	t.Helper()
	if len(jobs) != 1 || jobs[0].id != "check" || jobs[0].line == 0 || len(jobs[0].steps) != 0 {
		t.Fatalf("reusable job = %+v", jobs)
	}
}

func TestActionsReaderPreservesAliasesMergesAndSources(t *testing.T) {
	t.Parallel()
	const file = ".github/workflows/check.yml"
	const body = `events: &events [workflow_dispatch, push, push]
step: &step
  run: |
    echo ready
    make ci
  env:
    GOTOOLCHAIN: &mode local
on: *events
jobs:
  zed:
    steps:
      - *step
      - <<: *step
        run: make vuln
        env: {GOTOOLCHAIN: *mode, COUNT: 2, ALLOWED: true}
  alpha:
    uses: owner/repo/file@v1
    steps: ignored
    env: []
  local:
    steps:
      - uses: owner/action@v1
`
	got, err := readActions(makeInput(map[string]string{file: body}))
	if err != nil || len(got) != 1 {
		t.Fatalf("files = %+v, %v", got, err)
	}
	if !slices.Equal(got[0].triggers, []string{"push", "workflow_dispatch"}) || len(got[0].jobs) != 3 {
		t.Fatalf("file = %+v", got[0])
	}
	jobs := got[0].jobs
	if jobs[0].id != "zed" || jobs[0].line != 10 || jobs[1].id != "alpha" || jobs[1].line != 16 ||
		jobs[2].id != "local" || jobs[2].line != 20 || len(jobs[0].steps) != 2 || len(jobs[1].steps) != 0 ||
		len(jobs[2].steps) != 1 {
		t.Fatalf("jobs = %+v", jobs)
	}
	first, second := jobs[0].steps[0], jobs[0].steps[1]
	if first.line != 12 || first.run != "echo ready\nmake ci\n" || first.env["GOTOOLCHAIN"] != "local" ||
		second.line != 13 || second.run != "make vuln" || second.env["GOTOOLCHAIN"] != "local" ||
		second.env["COUNT"] != "2" || second.env["ALLOWED"] != "true" {
		t.Fatalf("steps = %+v", jobs[0].steps)
	}
	if jobs[2].steps[0].line != 22 || jobs[2].steps[0].uses != "owner/action@v1" {
		t.Fatalf("action step = %+v", jobs[2].steps[0])
	}
	first.env["GOTOOLCHAIN"] = "changed"
	if second.env["GOTOOLCHAIN"] != "local" {
		t.Fatal("steps share their environment map")
	}
}

func TestActionsReaderSortsMappingTriggersAndSkipsUnrelatedPaths(t *testing.T) {
	t.Parallel()
	const file = ".github/workflows/check.yaml"
	files := map[string]string{
		file:                                 "on:\n  workflow_dispatch:\n  push:\n  schedule:\n    - cron: '0 0 * * *'\n",
		".github/workflows/ignored.txt":      "not: [valid",
		".github/workflows/deep/ignored.yml": "not: [valid",
		"other/check.yml":                    "not: [valid",
	}
	got, err := readActions(makeInput(files))
	if err != nil || len(got) != 1 || got[0].path != file ||
		!slices.Equal(got[0].triggers, []string{"push", "schedule", "workflow_dispatch"}) {
		t.Fatalf("files = %+v, %v", got, err)
	}
}

func TestActionsReaderNamesUnsupportedRelevantForms(t *testing.T) {
	t.Parallel()
	const file = ".github/workflows/check.yml"
	for _, tc := range []struct {
		name, body, location, construct string
	}{
		{"event mapping in sequence", "on: [push, {event: schedule}]\n", ":1:", "on"},
		{"dynamic event in sequence", "on: [push, '${{ inputs.event }}']\n", ":1:", "on"},
		{"jobs sequence", "on: push\njobs: []\n", ":2:", "jobs"},
		{"scalar job", "jobs:\n  check: make ci\n", ":2:", "check"},
		{"steps mapping", "jobs:\n  check:\n    steps: {}\n", ":3:", "steps"},
		{"scalar step", "jobs:\n  check:\n    steps:\n      - make ci\n", ":4:", "step"},
		{"uses sequence", "jobs:\n  check:\n    steps:\n      - uses: [owner/action@v1]\n", ":4:", "uses"},
		{"environment sequence", "jobs:\n  check:\n    steps:\n      - run: make ci\n        env: []\n", ":5:", "env"},
		{"environment value sequence", "jobs:\n  check:\n    steps:\n      - run: make ci\n        env:\n          GOTOOLCHAIN: [local]\n", ":6:", "GOTOOLCHAIN"},
		{"duplicate run", "jobs:\n  check:\n    steps:\n      - {run: make ci, run: echo ready}\n", ":4:", "duplicate"},
		{"duplicate environment name", "jobs:\n  check:\n    steps:\n      - run: make ci\n        env: {COUNT: 1, COUNT: 2}\n", ":5:", "duplicate"},
		{"duplicate job", "jobs:\n  check: {steps: []}\n  check: {steps: []}\n", ":3:", "duplicate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := readActions(makeInput(map[string]string{file: tc.body}))
			if !errors.Is(err, model.ErrOutsideSubset) || !strings.Contains(err.Error(), file+tc.location) ||
				!strings.Contains(err.Error(), tc.construct) {
				t.Fatalf("limit = %v, want %s%s and %q", err, file, tc.location, tc.construct)
			}
		})
	}
}

func TestActionsReaderRetainsYAMLErrorSources(t *testing.T) {
	t.Parallel()
	const file = ".github/workflows/check.yml"
	for _, tc := range []struct {
		name, body string
		limited    bool
	}{
		{"malformed", "jobs: [", false},
		{"second document", "on: push\n---\non: schedule\n", true},
		{"sequence root", "- on: push\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := readActions(makeInput(map[string]string{file: tc.body}))
			if err == nil || !strings.Contains(err.Error(), file) || errors.Is(err, model.ErrOutsideSubset) != tc.limited {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestActionsReaderSelectsOnlyItsSnapshotFiles(t *testing.T) {
	t.Parallel()
	const file = ".github/workflows/check.yml"
	snapshot := &buildSelectingSnapshot{Tree: proof.NewTree(map[string][]byte{
		file: []byte("on: push\n"), "unrelated.yml": []byte("invalid: ["),
	})}
	got, err := readActions(model.Input{Snapshot: snapshot})
	if err != nil || len(got) != 1 || got[0].path != file {
		t.Fatalf("selected files = %+v, %v", got, err)
	}
	if snapshot.selectCalls != 1 || snapshot.filesCalls != 0 {
		t.Fatalf("selection called Select %d times and Files %d times", snapshot.selectCalls, snapshot.filesCalls)
	}
}

type actionsReadFailure struct {
	model.Snapshot

	failure error
}

func (s actionsReadFailure) Read(path string) ([]byte, error) {
	return nil, fmt.Errorf("%s: %w", path, s.failure)
}

func TestActionsReaderPreservesSnapshotReadOutcomes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		file    string
		failure error
	}{
		{".github/workflows/unreadable.yml", model.ErrUnreadable},
		{".github/workflows/large.yaml", model.ErrTooLarge},
		{".github/workflows/missing.yml", model.ErrNotInSnapshot},
	} {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			in := makeInput(map[string]string{tc.file: "on: push\n"})
			in.Snapshot = actionsReadFailure{Snapshot: in.Snapshot, failure: tc.failure}
			got, err := readActions(in)
			if len(got) != 0 {
				t.Fatalf("unread file supplied evidence: %+v", got)
			}
			requireActionsReadOutcome(t, err, tc.failure, tc.file)
		})
	}
}

func requireActionsReadOutcome(t *testing.T, err, failure error, file string) {
	t.Helper()
	if errors.Is(failure, model.ErrNotInSnapshot) {
		if err != nil {
			t.Fatalf("absent file = %v", err)
		}
		return
	}
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), file) {
		t.Fatalf("read error = %v, want %v at %s", err, failure, file)
	}
}
