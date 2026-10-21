package main

import (
	"os"
	"path/filepath"
	"slices"
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

// runnerApp is the enrolled app on a pipeline runner whose environment
// names only the variables kept, so a test sees which directories a scan
// resolves.
func runnerApp(t *testing.T, kept ...string) testApp {
	t.Helper()
	ta := enrolledApp(t)
	b := ta.bind()
	env := b.env
	b.env = func(name string) string {
		if slices.Contains(kept, name) {
			return env(name)
		}
		return ""
	}
	ta.bind = func() binding { return b }
	ta.snapshot, ta.load = b.snapshot, b.load
	return ta
}

func TestScanEnrolledRunsUnderTheProvisionedDigests(t *testing.T) {
	t.Parallel()
	ta := enrolledApp(t)
	root := repository(t, true)
	coreAssets(t, root)
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
	coreAssets(t, root)
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

// The index and the worktree hold opposite bytes, so only a scan of the
// index answers both cases.
func TestScanStagedEnrolledJudgesTheIndexUnderTheProvisionedDigests(t *testing.T) {
	t.Parallel()
	digest := digestOf(renderedPolicy(t))
	ta := enrolledApp(t)
	root := repository(t, true)
	stageFiles(t, root, map[string]string{"notes.md": "a " + stagedDash() + " b\n"})
	writeFiles(t, root, map[string]string{"notes.md": "a b\n"})
	if got := ta.run(t.Context(), []string{"scan", "--staged", "--enrolled", digest, root}); got != exitFindings || !strings.Contains(ta.stdout.String(), "notes.md:1:3 deny text.dashes") {
		t.Errorf("a dash only in the index: exit %d, stdout %q, stderr %q", got, ta.stdout.String(), ta.stderr.String())
	}
	clean := enrolledApp(t)
	cleanRoot := repository(t, true)
	writeFiles(t, cleanRoot, map[string]string{"draft.md": "c " + stagedDash() + " d\n"})
	if got := clean.run(t.Context(), []string{"scan", "--staged", "--enrolled", digest, cleanRoot}); got != exitClean || !strings.Contains(clean.stdout.String(), "verdict clean") {
		t.Errorf("a dash only in an untracked worktree file: exit %d, stdout %q, stderr %q", got, clean.stdout.String(), clean.stderr.String())
	}
	other := enrolledApp(t)
	if got := other.run(t.Context(), []string{"scan", "--staged", "--enrolled", otherDigest(), root}); got != exitFailure {
		t.Errorf("a digest the pipeline does not expect: exit %d", got)
	}
}

// A runner the pipeline provisions may name no home: the enrollment it
// provisions reads no record, so no state directory is resolved, and a
// policy without a profile reads no config directory either. A scan that
// reads the record still resolves the state directory.
func TestScanEnrolledResolvesOnlyTheDirectoriesItReads(t *testing.T) {
	t.Parallel()
	root := repository(t, true)
	coreAssets(t, root)
	digest := digestOf(renderedPolicy(t))
	for name, kept := range map[string][]string{"the config directory alone": {"XDG_CONFIG_HOME"}, "no variable": nil} {
		for _, args := range [][]string{{"scan", "--enrolled", digest, root}, {"scan", "--staged", "--enrolled", digest, root}} {
			ta := runnerApp(t, kept...)
			if got := ta.run(t.Context(), args); got != exitClean || !strings.Contains(ta.stdout.String(), "verdict clean") {
				t.Errorf("%s, %s: exit %d, stdout %q, stderr %q", name, args[1], got, ta.stdout.String(), ta.stderr.String())
			}
		}
	}
	local := runnerApp(t, "XDG_CONFIG_HOME")
	if got := local.run(t.Context(), []string{"scan", root}); got != exitFailure || !strings.Contains(local.stderr.String(), "neither XDG_STATE_HOME nor HOME is set") {
		t.Errorf("a scan that reads the record: exit %d, stderr %q", got, local.stderr.String())
	}
}

// A policy that names a profile reads the config directory: without one
// the refusal names the variables to set, and with one the bundle is
// verified as every load verifies it.
func TestScanEnrolledNamesTheConfigDirectoryAProfileReads(t *testing.T) {
	t.Parallel()
	profile := strings.Repeat("d", 64)
	line := `profile: { name: acme, source: https://example.com/acme.git, version: 0.3.0, digest: "sha256:` + profile + `" }`
	data := strings.Replace(string(renderedPolicy(t)), "language:", line+"\nlanguage:", 1)
	root := repository(t, false)
	writeFiles(t, root, map[string]string{"golaw.yml": data})
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "build: the policy")
	expected := digestOf([]byte(data)) + "," + profile
	cases := map[string]struct {
		kept []string
		says string
	}{
		"no variable":      {nil, "profile acme: resolve the config directory: usage: neither XDG_CONFIG_HOME nor HOME is set"},
		"the config named": {[]string{"XDG_CONFIG_HOME"}, "profile acme is not installed: run golaw profile install https://example.com/acme.git@0.3.0"},
	}
	for name, tc := range cases {
		ta := runnerApp(t, tc.kept...)
		if got := ta.run(t.Context(), []string{"scan", "--enrolled", expected, root}); got != exitFailure || !strings.Contains(ta.stderr.String(), tc.says) {
			t.Errorf("%s: exit %d, stderr %q", name, got, ta.stderr.String())
		}
	}
}
