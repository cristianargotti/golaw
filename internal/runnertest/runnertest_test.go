package runnertest_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runnertest"
)

func TestMain(m *testing.M) { os.Exit(runnertest.Main(m)) }

// played runs a fake tool and answers its streams and exit status.
func played(t *testing.T, path string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), path, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if exit, exited := errors.AsType[*exec.ExitError](err); exited {
		return stdout.String(), stderr.String(), exit.ExitCode()
	}
	if err != nil {
		t.Fatal(err)
	}
	return stdout.String(), stderr.String(), 0
}

func TestInstallPlaysTheToolItNames(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := runnertest.Install(t, dir, "golangci-lint", runnertest.Behavior{
		Probe: []string{"version"}, Version: "golangci-lint has version 2.14.0",
		Stdout: "linted\n", Stderr: "warned\n", Code: 3,
	})
	name := "golangci-lint"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if path != filepath.Join(dir, name) {
		t.Errorf("path = %s, want %s", path, filepath.Join(dir, name))
	}
	if stdout, _, code := played(t, path, "version"); stdout != "golangci-lint has version 2.14.0\n" || code != 0 {
		t.Errorf("probe = %q, %d", stdout, code)
	}
	if stdout, stderr, code := played(t, path, "run", "./..."); stdout != "linted\n" || stderr != "warned\n" || code != 3 {
		t.Errorf("run = %q, %q, %d", stdout, stderr, code)
	}
	want := [][]string{{"version"}, {"run", "./..."}}
	if calls := runnertest.Calls(t, dir, "golangci-lint"); !slices.EqualFunc(calls, want, slices.Equal) {
		t.Errorf("calls = %v, want %v", calls, want)
	}
}

func TestASecondInstallReplacesTheAnswerAndTheRecord(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	runnertest.Install(t, dir, "gitleaks", runnertest.Behavior{Code: 1})
	path := runnertest.Install(t, dir, "gitleaks", runnertest.Behavior{Stdout: "clean\n"})
	if stdout, _, code := played(t, path, "git", "--staged"); stdout != "clean\n" || code != 0 {
		t.Errorf("replaced fake = %q, %d", stdout, code)
	}
	if calls := runnertest.Calls(t, dir, "gitleaks"); len(calls) != 1 || !slices.Equal(calls[0], []string{"git", "--staged"}) {
		t.Errorf("calls = %v", calls)
	}
	if calls := runnertest.Calls(t, dir, "lefthook"); len(calls) != 0 {
		t.Errorf("a tool never started has no call, got %v", calls)
	}
}

func TestAFakeWhoseBehaviorBrokeSaysSo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := runnertest.Install(t, dir, "lefthook", runnertest.Behavior{})
	behavior := strings.TrimSuffix(path, ".exe") + ".behavior.json"
	if err := os.WriteFile(behavior, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := played(t, path, "install"); code != runnertest.BrokenCode || !strings.Contains(stderr, "behavior") {
		t.Errorf("a fake without its behavior = %q, %d", stderr, code)
	}
}

// fatal records a failure instead of ending the test, so a test proves that
// a helper fails the test it belongs to.
type fatal struct {
	testing.TB

	failed bool
}

func (f *fatal) Fatalf(string, ...any) { f.failed = true }

func TestHelpersFailTheTestTheyBelongTo(t *testing.T) {
	t.Parallel()
	f := &fatal{TB: t}
	runnertest.Install(f, filepath.Join(t.TempDir(), "absent"), "gitleaks", runnertest.Behavior{})
	if !f.failed {
		t.Error("installing into a missing directory fails the test")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gitleaks.calls.jsonl"), []byte("{\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.failed = false
	runnertest.Calls(f, dir, "gitleaks")
	if !f.failed {
		t.Error("a broken record fails the test")
	}
	if err := os.Mkdir(filepath.Join(dir, "lefthook.calls.jsonl"), 0o750); err != nil {
		t.Fatal(err)
	}
	f.failed = false
	runnertest.Calls(f, dir, "lefthook")
	if !f.failed {
		t.Error("an unreadable record fails the test")
	}
}
