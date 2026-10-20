package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestInitOwnershipAllowsUnchangedProtectedPolicy(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"apply", "preview", "update", "allowed template", "no module", "update no module"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			requireUnchangedProtectedPolicy(t, mode)
		})
	}
}

func requireUnchangedProtectedPolicy(t *testing.T, mode string) {
	t.Helper()
	repo, before := protectedPolicyFixture(t)
	args := []string{"init"}
	if strings.HasPrefix(mode, "update") {
		args = append(args, "--update")
	}
	if mode != "preview" {
		args = append(args, "--apply")
	}
	if strings.HasSuffix(mode, "no module") {
		repo.Remove("go.mod")
	}
	if mode == "allowed template" {
		repo.Remove(".editorconfig")
	}
	ta := initBindApp(t)
	if code := ta.run(t.Context(), append(args, repo.Root)); code != exitClean {
		t.Fatalf("unchanged protected policy refused: exit=%d: %s", code, ta.stderr.String())
	}
	got, err := bootstrap.Current(repo.Root)
	if err != nil || !bytes.Equal(got, before) {
		t.Fatalf("protected policy changed: %q, %v", got, err)
	}
	if !strings.Contains(ta.stdout.String(), "Destino protegido por CODEOWNERS, preservado: golaw.yml.") {
		t.Fatalf("protected policy notice missing: %q", ta.stdout.String())
	}
	requireOwnershipEnrollment(t, ta, repo.Root, before, mode != "preview")
	if _, err := os.Stat(filepath.Join(repo.Root, ".editorconfig")); err != nil {
		t.Fatalf("allowed template missing: %v", err)
	}
	requireUnarmed(t, repo.GitDir())
}

func TestInitOwnershipRefusesChangedProtectedPolicy(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"managed entry", "pin"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			requireChangedProtectedPolicyRefused(t, mode)
		})
	}
}

func requireChangedProtectedPolicyRefused(t *testing.T, mode string) {
	t.Helper()
	repo, original := protectedPolicyFixture(t)
	before := changeProtectedPolicy(t, original, mode)
	repo.Write(bootstrap.PolicyFile, string(before))
	repo.Remove(".editorconfig")
	snapshot := ownershipTree(t, repo.Root)
	ta := initBindApp(t)
	args, recovery := []string{"init", "--apply", repo.Root}, "golaw init --apply"
	if mode == "pin" {
		args, recovery = []string{"init", "--update", "--apply", repo.Root}, "golaw init --update --apply"
	}
	if code := ta.run(t.Context(), args); code != exitFailure {
		t.Fatalf("changed protected policy accepted: exit=%d: %s", code, ta.stderr.String())
	}
	for _, want := range []string{"CODEOWNERS", "golaw.yml", "owners", recovery, "commit golaw.yml"} {
		if !strings.Contains(ta.stderr.String(), want) {
			t.Errorf("refusal omits %q: %s", want, ta.stderr.String())
		}
	}
	if ta.stdout.Len() != 0 || !maps.Equal(snapshot, ownershipTree(t, repo.Root)) {
		t.Fatalf("refused initialization wrote output or files: %q", ta.stdout.String())
	}
	requireOwnershipEnrollment(t, ta, repo.Root, before, false)
	requireUnarmed(t, repo.GitDir())
}

func protectedPolicyFixture(t *testing.T) (*gittest.Repo, []byte) {
	t.Helper()
	repo := gittest.New(t)
	repo.Write("go.mod", "module example.test/service\n\ngo 1.27\n")
	ta := initBindApp(t)
	if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("fixture init exit=%d: %s", code, ta.stderr.String())
	}
	repo.Git("add", "--", ".")
	repo.Commit("chore: establish templates", nil)
	repo.Commit("chore: protect reviewed policy", map[string]string{
		"CODEOWNERS": "* @core\n/golaw.yml @platform\n",
	})
	data, err := bootstrap.Current(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	return repo, data
}

func changeProtectedPolicy(t *testing.T, data []byte, mode string) []byte {
	t.Helper()
	config, err := policy.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	pin, managed := config.Kit.Version, config.Managed
	if mode == "pin" {
		pin = "9.9.8"
	} else {
		if len(managed) == 0 {
			t.Fatal("fixture has no managed entries")
		}
		managed = managed[1:]
	}
	changed, err := bootstrap.ManagedPolicy(data, pin, managed)
	if err != nil || bytes.Equal(changed, data) {
		t.Fatalf("fixture policy unchanged: %v", err)
	}
	return changed
}

func requireOwnershipEnrollment(t *testing.T, ta testApp, root string, data []byte, want bool) {
	t.Helper()
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	record, found, err := policy.ReadRecord(dirs.State, root)
	if err != nil || found != want || want && record.Bootstrap != string(data) {
		t.Fatalf("policy enrollment = %+v, found=%v, want=%v, err=%v", record, found, want, err)
	}
}

type ownershipFileSnapshot struct {
	data     string
	modified time.Time
}

func ownershipTree(t *testing.T, root string) map[string]ownershipFileSnapshot {
	t.Helper()
	result := make(map[string]ownershipFileSnapshot)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read ownership snapshot: %w", readErr)
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return fmt.Errorf("stat ownership snapshot: %w", statErr)
		}
		result[path] = ownershipFileSnapshot{data: string(data), modified: info.ModTime()}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
