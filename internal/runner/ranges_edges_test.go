package runner_test

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

func TestRangesSkipsSHA256DeletionWithoutGit(t *testing.T) {
	dir := t.TempDir()
	runnertest.Install(t, dir, "git", runnertest.Behavior{Code: 2})
	t.Setenv("PATH", dir)
	update := runner.RefUpdate{
		LocalRef: "(delete)", LocalSHA: strings.Repeat("0", 64),
		RemoteRef: "refs/heads/gone", RemoteSHA: strings.Repeat("b", 64),
	}
	got, err := runner.Ranges(t.Context(), t.TempDir(), "origin", []runner.RefUpdate{update}, nil)
	if err != nil || len(got) != 1 {
		t.Fatalf("ranges = %+v, error = %v", got, err)
	}
	if got[0].Update != update || !got[0].Delete || got[0].Base != "" || got[0].Tip != update.LocalSHA {
		t.Fatalf("deletion = %+v", got[0])
	}
	if calls := runnertest.Calls(t, dir, "git"); len(calls) != 0 {
		t.Fatalf("deletion ran git: %v", calls)
	}
}

func TestRangesRejectsObjectArgumentsBeforeGit(t *testing.T) {
	dir := t.TempDir()
	runnertest.Install(t, dir, "git", runnertest.Behavior{Code: 2})
	t.Setenv("PATH", dir)
	//nolint:paralleltest // subtests share PATH and the fake git call record
	for _, tc := range []struct{ name, local, remote string }{
		{"red-local-option", "--help", runner.ZeroSHA},
		{"red-remote-option", strings.Repeat("a", 40), "--help"},
		{"red-deletion-remote", runner.ZeroSHA, "HEAD"},
		{"red-short-zero", "0000", runner.ZeroSHA},
		{"red-empty-local", "", runner.ZeroSHA},
	} {
		t.Run(tc.name, func(t *testing.T) {
			update := runner.RefUpdate{LocalRef: "HEAD", LocalSHA: tc.local, RemoteRef: "refs/heads/topic", RemoteSHA: tc.remote}
			got, err := runner.Ranges(t.Context(), t.TempDir(), "origin", []runner.RefUpdate{update}, nil)
			if !errors.Is(err, runner.ErrRefUpdate) || len(got) != 0 {
				t.Fatalf("ranges = %+v, error = %v", got, err)
			}
			if !strings.Contains(err.Error(), update.LocalRef) || !strings.Contains(err.Error(), update.RemoteRef) {
				t.Fatalf("error does not identify the update: %v", err)
			}
		})
	}
	if calls := runnertest.Calls(t, dir, "git"); len(calls) != 0 {
		t.Fatalf("malformed objects ran git: %v", calls)
	}
}

func TestRangesRejectsMalformedVerifierAnswers(t *testing.T) {
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	object := strings.Repeat("a", 40)
	for _, tc := range []struct {
		name, stdout, stderr, reason string
		code                         int
	}{
		{"red-empty", "", "", "invalid object", 0},
		{"red-zero", runner.ZeroSHA + "\n", "", "invalid object", 0},
		{"red-short", "abc\n", "", "invalid object", 0},
		{"red-many", object + "\n" + object + "\n", "", "invalid object", 0},
		{"red-absence-output", object + "\n", "", "exited 1", 1},
		{"red-absence-diagnostic", "", "object store unavailable", "object store unavailable", 1},
		{"red-command-failure", "", "repository unavailable", "exited 128", 128},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			runnertest.Install(t, dir, "git", runnertest.Behavior{Stdout: tc.stdout, Stderr: tc.stderr, Code: tc.code})
			t.Setenv("PATH", dir)
			update := runner.RefUpdate{LocalRef: "HEAD", LocalSHA: object, RemoteRef: "refs/heads/topic", RemoteSHA: runner.ZeroSHA}
			got, err := runner.Ranges(t.Context(), t.TempDir(), "origin", []runner.RefUpdate{update}, nil)
			if err == nil || len(got) != 0 {
				t.Fatalf("ranges = %+v, error = %v", got, err)
			}
			for _, text := range []string{update.LocalRef, update.RemoteRef, tc.reason} {
				if !strings.Contains(err.Error(), text) {
					t.Fatalf("error = %v, want %q", err, text)
				}
			}
			want := [][]string{{"-c", "core.quotePath=false", "rev-parse", "--verify", "--quiet", "--end-of-options", object + "^{commit}"}}
			if calls := runnertest.Calls(t, dir, "git"); !slices.EqualFunc(calls, want, slices.Equal) {
				t.Fatalf("calls = %v, want %v", calls, want)
			}
		})
	}
}

func TestRangesPreservesStartFailureAndRefIdentity(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	update := runner.RefUpdate{
		LocalRef: "refs/heads/topic", LocalSHA: strings.Repeat("a", 40),
		RemoteRef: "refs/heads/published", RemoteSHA: runner.ZeroSHA,
	}
	got, err := runner.Ranges(t.Context(), t.TempDir(), "origin", []runner.RefUpdate{update}, nil)
	if !errors.Is(err, runner.ErrStart) || !errors.Is(err, exec.ErrNotFound) || len(got) != 0 {
		t.Fatalf("ranges = %+v, error = %v", got, err)
	}
	if !strings.Contains(err.Error(), update.LocalRef) || !strings.Contains(err.Error(), update.RemoteRef) {
		t.Fatalf("error does not identify the update: %v", err)
	}
}

func TestRangesHidesRemoteCredentialsOnNamespaceFailure(t *testing.T) {
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	remote := "https://user:secret-token@example.test/repository.git"
	update := runner.RefUpdate{
		LocalRef: "refs/heads/topic", LocalSHA: strings.Repeat("a", 40),
		RemoteRef: "refs/heads/published", RemoteSHA: runner.ZeroSHA,
	}
	probe := []string{"-c", "core.quotePath=false", "rev-parse", "--verify", "--quiet", "--end-of-options", update.LocalSHA + "^{commit}"}
	for _, tc := range []struct {
		name, stdout, stderr string
		code                 int
	}{
		{"red-status", "", "failed to check " + remote, 128},
		{"red-stderr", "", "unexpected " + remote, 0},
		{"red-stdout", remote, "", 0},
		{"red-absent-output", "", remote, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			runnertest.Install(t, dir, "git", runnertest.Behavior{
				Probe: probe, Version: update.LocalSHA,
				Stdout: tc.stdout, Stderr: tc.stderr, Code: tc.code,
			})
			t.Setenv("PATH", dir)
			got, err := runner.Ranges(t.Context(), t.TempDir(), remote, []runner.RefUpdate{update}, nil)
			if len(got) != 0 {
				t.Fatalf("unexpected ranges: %+v", got)
			}
			requirePrivateRemoteError(t, err, remote, update)
			if !strings.Contains(err.Error(), "tracking namespace") {
				t.Fatalf("namespace operation missing: %v", err)
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("exited %d", tc.code)) {
				t.Fatalf("namespace status missing: %v", err)
			}
			want := [][]string{probe, {"-c", "core.quotePath=false", "check-ref-format", "refs/remotes/" + remote + "/HEAD"}}
			if calls := runnertest.Calls(t, dir, "git"); !slices.EqualFunc(calls, want, slices.Equal) {
				t.Fatal("expected tip verification followed by namespace check")
			}
		})
	}
}

func TestRangesHidesRemoteCredentialsWhenCancelled(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	tip := repo.Commit("chore: seed", map[string]string{"a.txt": "a\n"})
	remote := "https://user:secret-token@example.test/repository.git"
	update := runner.RefUpdate{
		LocalRef: "refs/heads/topic", LocalSHA: tip,
		RemoteRef: "refs/heads/published", RemoteSHA: runner.ZeroSHA,
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := runner.Ranges(ctx, repo.Root, remote, []runner.RefUpdate{update}, gittest.Isolated())
	if !errors.Is(err, context.Canceled) || len(got) != 0 {
		t.Fatalf("cancelled ranges = %+v, %v", got, err)
	}
	requirePrivateRemoteError(t, err, remote, update)
}

func requirePrivateRemoteError(t *testing.T, err error, remote string, update runner.RefUpdate) {
	t.Helper()
	if err == nil {
		t.Fatal("missing range error")
	}
	if strings.Contains(err.Error(), remote) || strings.Contains(err.Error(), "secret-token") {
		t.Fatal("range error exposes remote credentials")
	}
	if !strings.Contains(err.Error(), update.LocalRef) || !strings.Contains(err.Error(), update.RemoteRef) {
		t.Fatalf("error does not identify the update: %v", err)
	}
}
