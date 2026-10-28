package runnertest_test

import (
	"bytes"
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/runnertest"
)

// fed runs a fake tool with bytes on its stdin and answers its stdout and
// how long it took.
func fed(t *testing.T, path, stdin string, args ...string) (string, time.Duration) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), path, args...)
	cmd.Stdin = strings.NewReader(stdin)
	start := time.Now()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("run the fake: %v", err)
	}
	return string(out), time.Since(start)
}

func TestAFakeRecordsTheBytesItReadWhenAsked(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := runnertest.Install(t, dir, "ruff", runnertest.Behavior{Stdout: "[]\n", Stdin: true})
	fed(t, path, "import os\n", "check", "--stdin-filename", "scripts/a.py", "-")
	fed(t, path, "", "check", "--stdin-filename", "scripts/b.py", "-")
	got := runnertest.Inputs(t, dir, "ruff")
	want := [][]byte{[]byte("import os\n"), {}}
	if !slices.EqualFunc(got, want, bytes.Equal) {
		t.Errorf("Inputs = %q, want %q", got, want)
	}
	if calls := runnertest.Calls(t, dir, "ruff"); len(calls) != 2 {
		t.Errorf("Calls = %v, want two", calls)
	}
}

func TestAFakeThatIsNotAskedRecordsNoInput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := runnertest.Install(t, dir, "hadolint", runnertest.Behavior{Stdout: "[]\n"})
	fed(t, path, "FROM scratch\n", "--format", "json", "-")
	if got := runnertest.Inputs(t, dir, "hadolint"); len(got) != 0 {
		t.Errorf("Inputs = %q, want none: the behavior did not ask for stdin", got)
	}
	if got := runnertest.Inputs(t, dir, "never-installed"); len(got) != 0 {
		t.Errorf("Inputs of a tool never installed = %q, want none", got)
	}
}

func TestAReinstallStartsANewInputRecord(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := runnertest.Install(t, dir, "shellcheck", runnertest.Behavior{Stdin: true})
	fed(t, path, "echo $1\n", "--format", "json1", "-")
	runnertest.Install(t, dir, "shellcheck", runnertest.Behavior{Stdin: true})
	if got := runnertest.Inputs(t, dir, "shellcheck"); len(got) != 0 {
		t.Errorf("Inputs after a reinstall = %q, want none", got)
	}
}

func TestAFakeWaitsItsDelayBeforeItAnswers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	delayed := runnertest.Install(t, dir, "slow", runnertest.Behavior{Stdout: "late\n", Delay: "400ms", Probe: []string{"--version"}, Version: "slow 1.0.0"})
	out, took := fed(t, delayed, "", "check")
	if out != "late\n" || took < 400*time.Millisecond {
		t.Errorf("delayed answer = %q after %v, want late after at least 400ms", out, took)
	}
	if version, _ := fed(t, delayed, "", "--version"); version != "slow 1.0.0\n" {
		t.Errorf("probe = %q, want the version: a delay holds only the answer, never the probe", version)
	}
}

func TestAFakeWithAnUnreadableDelayIsBroken(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := runnertest.Install(t, dir, "broken", runnertest.Behavior{Delay: "soon"})
	err := exec.CommandContext(t.Context(), path, "check").Run()
	exit, ok := errors.AsType[*exec.ExitError](err)
	if !ok || exit.ExitCode() != runnertest.BrokenCode {
		t.Errorf("run = %v, want exit %d", err, runnertest.BrokenCode)
	}
}
