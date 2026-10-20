package main

import (
	"errors"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

//nolint:paralleltest // The command deliberately uses the repository cwd.
func TestLockCommandPassesStreamsStatusAndReleases(t *testing.T) {
	repo := gittest.New(t)
	t.Chdir(repo.Root)
	for _, tc := range []struct {
		name  string
		code  int
		large bool
	}{
		{"near-success", 0, false},
		{"red-child-status", 7, false},
		{"near-large-streams", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			stdout, stderr := "result\n", "detail\n"
			if tc.large {
				stdout = strings.Repeat("x", runner.MaxOutputBytes+1)
				stderr = strings.Repeat("e", runner.MaxOutputBytes+1)
			}
			program := runnertest.Install(t, dir, "probe", runnertest.Behavior{
				Stdout: stdout, Stderr: stderr, Code: tc.code,
			})
			ta := ladderApp(t)
			requireLockedRun(t, ta, program, dir, stdout, stderr, tc.code)
			requireLockReleased(t, repo.GitDir())
		})
	}
}

// requireLockedRun requires golaw lock to pass the arguments, both streams
// whole and the status of the child.
func requireLockedRun(t *testing.T, ta testApp, program, dir, stdout, stderr string, code int) {
	t.Helper()
	args := []string{"lock", "measure", "--", program, "--value", "a b"}
	if got := ta.run(t.Context(), args); got != code {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	if ta.stdout.String() != stdout || ta.stderr.String() != stderr {
		t.Fatalf("stdout bytes=%d stderr bytes=%d",
			ta.stdout.Len(), ta.stderr.Len())
	}
	calls := runnertest.Calls(t, dir, "probe")
	if !slices.EqualFunc(calls, [][]string{{"--value", "a b"}}, slices.Equal) {
		t.Fatalf("arguments = %v", calls)
	}
}

// requireLockReleased requires the lock of the measured command free once
// golaw lock returned.
func requireLockReleased(t *testing.T, gitDir string) {
	t.Helper()
	lock, err := runner.Acquire(
		t.Context(), runner.LockPath(gitDir, "measure"), 0,
	)
	if err != nil {
		t.Fatal(err)
	}
	if releaseErr := lock.Release(); releaseErr != nil {
		t.Fatal(releaseErr)
	}
}

func TestLockedAcceptsOnlyASCIILockNames(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"A", "z", "0", "a.Z_0-", "0a..b"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			dir := t.TempDir()
			program := runnertest.Install(t, dir, "probe", runnertest.Behavior{
				Probe: []string{"version"},
			})
			b := ladderApp(t).bind()
			code, err := b.locked(t.Context(), repo.Root, name,
				[]string{program}, nil, io.Discard, io.Discard)
			if err != nil || code != 0 || len(runnertest.Calls(t, dir, "probe")) != 1 {
				t.Fatalf("name=%q code=%d error=%v", name, code, err)
			}
			lock, err := runner.Acquire(t.Context(), runner.LockPath(repo.GitDir(), name), 0)
			if err != nil {
				t.Fatal(err)
			}
			if err := lock.Release(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLockedValidatesBeforeDiscoveringOrDerivingAPath(t *testing.T) {
	t.Parallel()
	b := ladderApp(t).bind()
	dir := t.TempDir()
	for _, name := range []string{
		"", ".", "..", "-name", "_name", "/outside", "a/b", `a\b`,
		"a b", "é", "aé", "a\nb", "a:b",
	} {
		_, err := b.locked(t.Context(), dir, name,
			[]string{"not-started"}, nil, io.Discard, io.Discard)
		if !errors.Is(err, model.ErrUsage) {
			t.Fatalf("invalid name %q reached discovery: %v", name, err)
		}
	}
	for _, args := range [][]string{nil, {}, {""}, {"", "argument"}} {
		_, err := b.locked(t.Context(), dir, "measure", args, nil, io.Discard, io.Discard)
		if !errors.Is(err, model.ErrUsage) {
			t.Fatalf("invalid command %q reached discovery: %v", args, err)
		}
	}
}

func TestLockedStartFailureReleasesTheLock(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	b := ladderApp(t).bind()
	program := filepath.Join(t.TempDir(), "missing-program")
	_, err := b.locked(t.Context(), repo.Root, "measure",
		[]string{program}, nil, io.Discard, io.Discard)
	if !errors.Is(err, runner.ErrStart) || !strings.Contains(err.Error(), program) {
		t.Fatalf("start error = %v", err)
	}
	requireLockReleased(t, repo.GitDir())
}
