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
	if snapshot.EnrollmentError == "" || snapshot.ClaudeVersion != ErrVersion.Error() {
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
