package rung_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// imageOnly is a policy whose repository holds no Go program.
const imageOnly = "golaw: 1\nkit: {version: 0.1.0}\ncapabilities: [image]\n"

// The lines of the Go steps while the go capability is off.
const (
	fmtOff   = "golaw pre-commit: fmt: skipped, the go capability is off"
	lintOff  = "golaw pre-commit: lint: skipped, the go capability is off"
	vetOff   = "golaw pre-commit: vet: skipped, the go capability is off"
	testOff  = "golaw pre-commit: test: skipped, the go capability is off"
	testIdle = "golaw pre-commit: test: skipped, test_staged is off in the ladder of golaw.yml"
)

//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitRunsNoGoToolWithTheGoCapabilityOff(t *testing.T) {
	cases := map[string]struct {
		ladder string
		test   string
	}{
		"test_staged off": {"", testIdle},
		"test_staged on":  {"ladder: {test_staged: true}\n", testOff},
	}
	for name, tc := range cases {
		dir := fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
		r := armedRepo(t, imageOnly+tc.ladder)
		r.Stage(map[string]string{"a/a.go": vetBroken})
		d := deps(t, r)
		out := rung.RunPreCommit(t.Context(), d)
		for _, line := range []string{fmtOff, lintOff, vetOff, tc.test} {
			if !contains(out, line) {
				t.Errorf("%s: no line %q in %+v", name, line, out)
			}
		}
		if out.Code != 0 || contains(out, "go vet reports") {
			t.Errorf("%s = %+v", name, out)
		}
		if calls := runnertest.Calls(t, dir, "golangci-lint"); len(calls) != 0 {
			t.Errorf("%s: golangci-lint calls = %v", name, calls)
		}
		if calls := runnertest.Calls(t, dir, "gitleaks"); len(calls) != 2 {
			t.Errorf("%s: the secrets step ran %d gitleaks calls, want the probe and the scan", name, len(calls))
		}
		if _, err := os.Stat(filepath.Join(d.State, "runner", "trees")); !os.IsNotExist(err) {
			t.Errorf("%s: a tree was materialized: %v", name, err)
		}
	}
}

//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitRunsTheGoStepsWhileGoHolds(t *testing.T) {
	dir := fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	r := armedRepo(t, "golaw: 1\nkit: {version: 0.1.0}\ncapabilities: [go, image]\n")
	r.Stage(map[string]string{"a/a.go": vetBroken})
	out := rung.RunPreCommit(t.Context(), deps(t, r))
	if out.Code != 1 || !contains(out, "golaw pre-commit: vet: go vet reports what the affected packages break") || contains(out, vetOff) {
		t.Errorf("go and image = %+v", out)
	}
	if calls := runnertest.Calls(t, dir, "golangci-lint"); len(calls) == 0 {
		t.Error("golangci-lint never ran while go holds")
	}
}

//nolint:paralleltest // the fake tools are found through PATH
func TestPrePushRunsNoScannerWithTheGoCapabilityOff(t *testing.T) {
	dir := fakes(t, map[string]runnertest.Behavior{"govulncheck": scanned("v1.7.0", depFinding)})
	r := armedRepo(t, imageOnly)
	out := rung.RunPrePush(t.Context(), pushing(t, r, update(r.Head())), "origin", "")
	if out.Code != 0 || len(out.Lines) != 1 || out.Lines[0] != "golaw pre-push: skipped, the go capability is off" {
		t.Errorf("go off = %+v", out)
	}
	if calls := runnertest.Calls(t, dir, "govulncheck"); len(calls) != 0 {
		t.Errorf("govulncheck calls with go off = %v", calls)
	}
	held := armedRepo(t, armedPolicy)
	if out := rung.RunPrePush(t.Context(), pushing(t, held, update(held.Head())), "origin", ""); out.Code != 1 || len(runnertest.Calls(t, dir, "govulncheck")) != 1 {
		t.Errorf("go on = %+v", out)
	}
}
