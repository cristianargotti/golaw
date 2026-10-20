package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/setup"
	"github.com/cristianargotti/golaw/internal/source"
)

func TestSetupUninstallNeedsNeitherGoNorPolicyWithoutReceipt(t *testing.T) {
	bin := t.TempDir()
	runnertest.Install(t, bin, "go", runnertest.Behavior{Code: 1, Stderr: "go must not run"})
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	repo := gittest.New(t)
	repo.Commit("chore: invalid policy", map[string]string{"golaw.yml": "invalid: [\n"})
	ta := bindApp(t)
	if code := ta.run(t.Context(), []string{"setup", "--uninstall", repo.Root}); code != exitClean ||
		ta.stdout.Len() != 0 || ta.stderr.Len() != 0 {
		t.Fatalf("uninstall = %d, stdout %q, stderr %q", code, ta.stdout.String(), ta.stderr.String())
	}
	if calls := runnertest.Calls(t, bin, "go"); len(calls) != 0 {
		t.Fatalf("uninstall started go: %v", calls)
	}
}

func TestSetupChecksBothPinsBeforeGoDiscovery(t *testing.T) {
	bin := t.TempDir()
	runnertest.Install(t, bin, "go", runnertest.Behavior{Code: 1, Stderr: "go must not run"})
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, version := range []string{"9.9.8", "9.9.10"} {
		repo := gittest.New(t)
		repo.Commit("chore: pin policy", map[string]string{
			"golaw.yml": strings.Replace(string(renderedPolicy(t)), testVersion, version, 1),
		})
		ta := bindApp(t)
		if code := ta.run(t.Context(), []string{"setup", repo.Root}); code != exitFailure ||
			!strings.Contains(ta.stderr.String(), version) {
			t.Fatalf("pin %s: exit %d, stderr %q", version, code, ta.stderr.String())
		}
	}
	if calls := runnertest.Calls(t, bin, "go"); len(calls) != 0 {
		t.Fatalf("pin mismatch started go: %v", calls)
	}
}

func TestSetupReportsDiscoveryAndStateFailures(t *testing.T) {
	t.Parallel()
	b := bindApp(t).bind()
	for _, uninstall := range []bool{false, true} {
		if _, err := b.setupPlan(t.Context(), t.TempDir(), now(), uninstall, setup.Dependencies{}); !errors.Is(err, source.ErrNotRepository) {
			t.Fatalf("discovery: %v", err)
		}
	}
	repo := gittest.New(t)
	b.env = func(string) string { return "" }
	if _, err := b.setupBaseOptions(t.Context(), repo.Root, now()); err == nil {
		t.Fatal("state directory failure was ignored")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := b.setupOptions(ctx, repo.Root, now()); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestSetupUninstallRefusesInvalidReceipt(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	ta := bindApp(t)
	options, err := ta.bind().setupBaseOptions(t.Context(), repo.Root, now())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(options.StateDir, "setup")
	if err = os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	// A receipt-shaped directory must be reported, never treated as absence.
	_, identity, err := install.ProjectIdentity(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(setup.ReceiptPath(options.StateDir, identity), 0o700); err != nil {
		t.Fatal(err)
	}
	if code := ta.run(t.Context(), []string{"setup", "--uninstall", repo.Root}); code != exitFailure ||
		!strings.Contains(ta.stderr.String(), "receipt") || ta.stdout.Len() != 0 {
		t.Fatalf("invalid receipt: exit %d, stdout %q, stderr %q", code, ta.stdout.String(), ta.stderr.String())
	}
}
