package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/policy"
)

// enrolledApp is the staged app with the worktree seams of the same
// binding, so both scans run over the production adapters.
func enrolledApp(t *testing.T) testApp {
	t.Helper()
	ta := stagedApp(t)
	b := ta.bind()
	ta.snapshot, ta.load = b.snapshot, b.load
	return ta
}

// otherDigest is a well formed digest no test policy has.
func otherDigest() string { return strings.Repeat("c", 64) }

func TestScanEnrolledRunsUnderTheProvisionedDigests(t *testing.T) {
	t.Parallel()
	ta := enrolledApp(t)
	root := repository(t, true)
	digest := digestOf(renderedPolicy(t))
	if got := ta.run(t.Context(), []string{"scan", "--enrolled", digest, root}); got != exitClean || !strings.Contains(ta.stdout.String(), "verdict clean") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", got, ta.stdout.String(), ta.stderr.String())
	}
	other := enrolledApp(t)
	if got := other.run(t.Context(), []string{"scan", "--enrolled", otherDigest(), root}); got != exitFailure {
		t.Fatalf("a digest the pipeline does not expect: exit %d", got)
	}
	if got := other.stderr.String(); !strings.Contains(got, digest) || !strings.Contains(got, otherDigest()) || !strings.Contains(got, "golaw config explain") {
		t.Errorf("stderr = %q", got)
	}
}

func TestScanEnrolledReadsNoRecordOfTheStateDirectory(t *testing.T) {
	t.Parallel()
	ta := enrolledApp(t)
	root := repository(t, true)
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	record := policy.Record{Root: root, PolicyDigest: otherDigest(), KitVersion: testVersion, WrittenBy: policy.WrittenByInit, WrittenAt: now()}
	if err := policy.WriteRecord(dirs.State, record); err != nil {
		t.Fatal(err)
	}
	if got := ta.run(t.Context(), []string{"scan", root}); got != exitFailure {
		t.Fatalf("the record of the state directory differs: exit %d", got)
	}
	if got := ta.run(t.Context(), []string{"scan", "--enrolled", digestOf(renderedPolicy(t)), root}); got != exitClean {
		t.Errorf("under the provisioned digests: exit %d, stderr %q", got, ta.stderr.String())
	}
}

func TestScanEnrolledRefusesAMissingPolicyAndAMalformedValue(t *testing.T) {
	t.Parallel()
	digest := digestOf(renderedPolicy(t))
	without := repository(t, false)
	gitRun(t, without, "add", "go.mod")
	gitRun(t, without, "commit", "-q", "-m", "build: the module")
	deleted := repository(t, true)
	if err := os.Remove(filepath.Join(deleted, "golaw.yml")); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		args []string
		says string
	}{
		"a HEAD without golaw.yml": {[]string{"scan", "--enrolled", digest, without}, "has no golaw.yml at HEAD"},
		"an unborn branch":         {[]string{"scan", "--enrolled", digest, repository(t, false)}, "has no golaw.yml at HEAD"},
		"a checkout without it":    {[]string{"scan", "--enrolled", digest, deleted}, "golaw.yml is gone from the checkout"},
		"a malformed value":        {[]string{"scan", "--enrolled", "sha256:abc", repository(t, true)}, "golaw: scan: usage: the expected enrollment"},
		"an empty value":           {[]string{"scan", "--enrolled=", repository(t, true)}, "golaw: scan: usage: the expected enrollment"},
	}
	for name, tc := range cases {
		ta := enrolledApp(t)
		if got := ta.run(t.Context(), tc.args); got != exitFailure || !strings.Contains(ta.stderr.String(), tc.says) {
			t.Errorf("%s: exit %d, stderr %q", name, got, ta.stderr.String())
		}
	}
}

func TestScanStagedEnrolledJudgesTheIndexUnderTheProvisionedDigests(t *testing.T) {
	t.Parallel()
	ta := enrolledApp(t)
	root := repository(t, true)
	stageFiles(t, root, map[string]string{"notes.md": "a " + stagedDash() + " b\n"})
	if got := ta.run(t.Context(), []string{"scan", "--staged", "--enrolled", digestOf(renderedPolicy(t)), root}); got != exitFindings {
		t.Errorf("exit = %d, stdout = %q, stderr = %q", got, ta.stdout.String(), ta.stderr.String())
	}
	other := enrolledApp(t)
	if got := other.run(t.Context(), []string{"scan", "--staged", "--enrolled", otherDigest(), root}); got != exitFailure {
		t.Errorf("a digest the pipeline does not expect: exit %d", got)
	}
}
