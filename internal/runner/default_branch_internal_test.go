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
