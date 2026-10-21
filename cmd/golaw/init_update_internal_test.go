package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestInitUpdateMovesThePinAndPreservesLocalEdits(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Write("go.mod", adoptedModule)
	ta := initBindApp(t)
	path := filepath.Join(repo.Root, "golaw.yml")
	older, before := olderPolicy(t, ta, repo, path)
	local := "title = \"local configuration\"\n\n[extend]\nuseDefault = true\n"
	repo.Write(".gitleaks.toml", local)
	requireUpdatePreview(t, ta, repo.Root, path, older)
	requireUpdateApplied(t, ta, repo.Root, before, local)
	requireSecondUpdate(t, ta, repo.Root)
}

// olderPolicy applies init, then pins golaw.yml to an older kit version,
// and answers its text and its decoded policy.
func olderPolicy(t *testing.T, ta testApp, repo *gittest.Repo, path string) (string, policy.File) {
	t.Helper()
	if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("initial exit = %d: %s", code, ta.stderr.String())
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	older := strings.Replace(string(current), testVersion, "9.9.8", 1)
	repo.Write("golaw.yml", older)
	before, err := policy.Decode([]byte(older))
	if err != nil {
		t.Fatal(err)
	}
	return older, before
}

// requireUpdatePreview requires init --update to keep golaw.yml and to
// name the pin and the local edit.
func requireUpdatePreview(t *testing.T, ta testApp, root, path, older string) {
	t.Helper()
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"init", "--update", root}); code != exitClean {
		t.Fatalf("preview exit = %d: %s", code, ta.stderr.String())
	}
	unchanged, err := os.ReadFile(path)
	if err != nil || string(unchanged) != older {
		t.Fatalf("preview changed policy: %v", err)
	}
	if !strings.Contains(ta.stdout.String(), testVersion) ||
		!strings.Contains(ta.stdout.String(), ".gitleaks.toml") {
		t.Fatalf("preview omitted pin or local edit: %q", ta.stdout.String())
	}
}

// requireUpdateApplied requires init --update --apply to move the pin and
// the digest, to keep the local edit, and to report it and ask for the
// commit.
func requireUpdateApplied(t *testing.T, ta testApp, root string, before policy.File, local string) {
	t.Helper()
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"init", "--update", "--apply", root}); code != exitClean {
		t.Fatalf("update exit = %d: %s", code, ta.stderr.String())
	}
	updated, err := os.ReadFile(filepath.Join(root, "golaw.yml"))
	if err != nil {
		t.Fatal(err)
	}
	after, err := policy.Decode(updated)
	if err != nil || after.Kit.Version != testVersion {
		t.Fatalf("updated policy = %+v, %v", after, err)
	}
	if policy.Digest(before) == policy.Digest(after) {
		t.Fatal("pin movement did not change the digest")
	}
	kept, err := os.ReadFile(filepath.Join(root, ".gitleaks.toml"))
	if err != nil || string(kept) != local {
		t.Fatalf("local edit was replaced: %q, %v", kept, err)
	}
	if !strings.Contains(ta.stdout.String(), ".gitleaks.toml") ||
		!strings.HasSuffix(ta.stdout.String(), commitAsked) {
		t.Fatalf("update report = %q", ta.stdout.String())
	}
}

// requireSecondUpdate requires a second update to print no diff and to ask
// for the commit.
func requireSecondUpdate(t *testing.T, ta testApp, root string) {
	t.Helper()
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"init", "--update", "--apply", root}); code != exitClean {
		t.Fatalf("second update exit = %d: %s", code, ta.stderr.String())
	}
	for _, marker := range []string{"+++ ", "--- ", "Gravações:"} {
		if strings.Contains(ta.stdout.String(), marker) {
			t.Fatalf("second update has a diff: %q", ta.stdout.String())
		}
	}
	if !strings.HasSuffix(ta.stdout.String(), commitAsked) {
		t.Fatalf("commit reminder missing: %q", ta.stdout.String())
	}
}

func TestInitUpdateRequiresAnExistingPolicy(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	ta := initBindApp(t)
	if code := ta.run(t.Context(), []string{"init", "--update", repo.Root}); code != exitFailure {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(ta.stderr.String(), "golaw init --apply") {
		t.Fatalf("missing recovery command: %q", ta.stderr.String())
	}
}
