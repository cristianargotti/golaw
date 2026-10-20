package runner

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestTrackingNamespaceHidesRemoteAndPreservesCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	remote := "https://user:secret-token@example.test/repository.git"
	git := rangeGit{dir: t.TempDir()}
	known, err := git.trackingNamespace(ctx, remote)
	if known || !errors.Is(err, context.Canceled) {
		t.Fatalf("namespace = %v, %v", known, err)
	}
	requirePrivateNamespaceError(t, err, remote)
}

func TestTrackingNamespaceHidesRemoteAndPreservesStartFailure(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	remote := "https://user:secret-token@example.test/repository.git"
	git := rangeGit{dir: t.TempDir()}
	known, err := git.trackingNamespace(t.Context(), remote)
	if known || !errors.Is(err, ErrStart) || !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("namespace = %v, %v", known, err)
	}
	requirePrivateNamespaceError(t, err, remote)
}

func requirePrivateNamespaceError(t *testing.T, err error, remote string) {
	t.Helper()
	if strings.Contains(err.Error(), remote) || strings.Contains(err.Error(), "secret-token") {
		t.Fatal("tracking namespace error exposes remote credentials")
	}
	if !strings.Contains(err.Error(), "tracking namespace") {
		t.Fatalf("namespace operation missing: %v", err)
	}
}

func TestTrackingObjectTakesAnAnswerWithoutObjectAsAbsent(t *testing.T) {
	t.Parallel()
	object := strings.Repeat("a", 40)
	args := []string{"rev-parse", "--verify", "--quiet", "--end-of-options", "refs/remotes/origin/HEAD^{commit}"}
	for _, tc := range []struct {
		name, want, reason string
		out                Output
	}{
		{"near-absent", "", "", Output{Code: 1}},
		{"near-dangling-symref", "", "", Output{Code: 1, Stderr: []byte("warning: ignoring dangling symref refs/remotes/origin/HEAD\n")}},
		{"near-broken-ref", "", "", Output{Code: 1, Stderr: []byte("warning: ignoring broken ref refs/remotes/origin/HEAD\n")}},
		// git translates its warnings, so the text of stderr is never read.
		{"near-any-diagnostic", "", "", Output{Code: 1, Stderr: []byte("object store unavailable")}},
		{"near-object", object, "", Output{Stdout: []byte(object + "\n")}},
		{"red-absence-output", "", "exited 1", Output{Code: 1, Stdout: []byte(object + "\n")}},
		{"red-command-failure", "", "exited 128", Output{Code: 128, Stderr: []byte("repository unavailable")}},
		{"red-invalid-object", "", "invalid object", Output{Stdout: []byte("abc\n")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := trackingObject(args, tc.out)
			if tc.reason == "" {
				if err != nil || got != tc.want {
					t.Fatalf("tracking object = %q, %v", got, err)
				}
				return
			}
			if err == nil || got != "" || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("tracking object = %q, %v, want %q", got, err, tc.reason)
			}
		})
	}
}
