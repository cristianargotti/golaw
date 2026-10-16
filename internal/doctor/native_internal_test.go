package doctor

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestMain lets the test binary stand in for the agent executable: asked
// for its version, it prints one and leaves.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		if _, err := os.Stdout.WriteString("2.1.299 (test double)\n"); err != nil {
			return
		}
		return
	}
	m.Run()
}

// doubleOnPath copies the test binary under the agent's name into a fresh
// directory and answers that directory.
func doubleOnPath(t *testing.T) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	name := agentClaude
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(dir, name), body, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestNativeRunnerReadsTheVersionThroughTheObservedPath(t *testing.T) {
	t.Parallel()
	dir := doubleOnPath(t)
	environment := AgentEnvironment{Source: sourceAgent, Path: dir, PathExt: ".EXE"}
	version, err := NativeRunner{}.Version(t.Context(), environment)
	if err != nil || !strings.Contains(version, "2.1.299") {
		t.Fatalf("version = %q, %v", version, err)
	}
	if _, err := (NativeRunner{}).Version(t.Context(), AgentEnvironment{Source: sourceAgent, Path: t.TempDir()}); err == nil {
		t.Fatal("an absent executable answered a version")
	}
}

func TestCollectReportsMissingEvidence(t *testing.T) {
	t.Parallel()
	receipt := filepath.Join(t.TempDir(), "none.json")
	input := Input{Root: filepath.Join(t.TempDir(), "absent"), ReceiptPath: receipt}
	snapshot := Collect(t.Context(), input, nil, &versionDouble{err: ErrVersion})
	if snapshot.EnrollmentError == "" || snapshot.ClaudeVersion != "" || snapshot.ClaudeVersionError != ErrVersion.Error() {
		t.Fatalf("missing root = %#v", snapshot)
	}
	input.Root = t.TempDir()
	if got := Collect(t.Context(), input, nil, nil); got.EnrollmentError != "enrollment reader unavailable" {
		t.Fatalf("nil reader = %#v", got)
	}
	store := &enrollmentStore{identity: "other"}
	if got := Collect(t.Context(), input, store, nil); got.EnrollmentError == "" {
		t.Fatalf("a foreign enrollment was accepted: %#v", got)
	}
}

func TestAgentVersionStaysUnknownUntilItIsRead(t *testing.T) {
	t.Parallel()
	input := Input{Root: t.TempDir(), ReceiptPath: filepath.Join(t.TempDir(), "none.json")}
	for _, tc := range []struct {
		name, state, code, detail string
		runner                    VersionRunner
	}{
		{"unobserved environment", stateUnknown, "agent-version-unavailable", ErrEnvironment.Error(), NativeRunner{}},
		{"no runner", stateUnknown, "agent-version-unavailable", "empty version", nil},
		{"unparsed output", stateUnknown, "agent-version-unavailable", "Claude Code", &versionDouble{output: "Claude Code"}},
		{"below the floor", stateRed, "agent-version-below-floor", "2.1.294", &versionDouble{output: "2.1.294 (Claude Code)"}},
		{"at the floor", stateOK, "agent-version-supported", "2.1.295", &versionDouble{output: "2.1.295 (Claude Code)"}},
	} {
		var line Line
		for _, candidate := range Assess(Collect(t.Context(), input, nil, tc.runner)).Lines {
			if candidate.ID == "doctor.version" {
				line = candidate
			}
		}
		if line.State != tc.state || line.Code != tc.code || !strings.Contains(line.Detail, tc.detail) {
			t.Errorf("%s: %#v", tc.name, line)
		}
	}
}
