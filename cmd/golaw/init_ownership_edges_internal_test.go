package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestInitOwnershipProtectsPolicyWithoutUsableModule(t *testing.T) {
	t.Parallel()
	for _, directive := range []string{"", "1.23", "1.27"} {
		t.Run("go "+directive, func(t *testing.T) {
			t.Parallel()
			for _, update := range []bool{false, true} {
				requireProtectedPolicyUntouched(t, directive, update)
			}
		})
	}
}

func requireProtectedPolicyUntouched(t *testing.T, directive string, update bool) {
	t.Helper()
	repo := gittest.New(t)
	repo.Write("CODEOWNERS", "* @core\n/golaw.yml @platform\n")
	if directive != "" {
		repo.Write("go.mod", "module example.test/service\n\ngo "+directive+"\n")
	}
	const reviewed = "golaw: 1\nkit: {version: 9.9.9}\n"
	args := []string{"init", "--apply", repo.Root}
	if update {
		repo.Write(bootstrap.PolicyFile, reviewed)
		args = []string{"init", "--update", "--apply", repo.Root}
	}
	ta := initBindApp(t)
	if code := ta.run(t.Context(), args); code != exitFailure {
		t.Fatalf("protected policy exit = %d, update=%v: %s", code, update, ta.stderr.String())
	}
	if !strings.Contains(ta.stderr.String(), "golaw.yml") || ta.stdout.Len() != 0 {
		t.Fatalf("protected policy output = %q, %q", ta.stdout.String(), ta.stderr.String())
	}
	got, err := bootstrap.Current(repo.Root)
	if err != nil || update && string(got) != reviewed || !update && got != nil {
		t.Fatalf("protected policy changed = %q, %v", got, err)
	}
	requireNoInitTemplates(t, repo.Root)
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	if _, found, readErr := policy.ReadRecord(dirs.State, repo.Root); readErr != nil || found {
		t.Fatalf("protected policy enrolled = %v, %v", found, readErr)
	}
}

func TestInitOwnershipNoticesKeepManagedBookkeepingAndIdempotence(t *testing.T) {
	t.Parallel()
	for _, owners := range []string{
		"* @core\n/.github/ @platform\n",
		"* @core\n!/unrelated/ @platform\n",
	} {
		t.Run(owners, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			repo.Write("go.mod", adoptedModule)
			repo.Write("CODEOWNERS", owners)
			ta := initBindApp(t)
			args := []string{"init", "--apply", repo.Root}
			if code := ta.run(t.Context(), args); code != exitClean {
				t.Fatal(ta.stderr.String())
			}
			times := managedTimes(t, repo.Root)
			if strings.Contains(owners, "/.github/") {
				requireProtectedAssetUnmanaged(t, repo.Root, times)
			}
			ta.stdout.Reset()
			if code := ta.run(t.Context(), args); code != exitClean {
				t.Fatal(ta.stderr.String())
			}
			requireSilentApply(t, ta.stdout.String(), repo.Root, times)
			if !strings.HasPrefix(ta.stdout.String(), "golaw.yml: no changes\n") {
				t.Fatalf("unchanged policy status lost: %q", ta.stdout.String())
			}
			requireUnarmed(t, repo.GitDir())
		})
	}
}

func requireProtectedAssetUnmanaged(t *testing.T, root string, times map[string]fs.FileInfo) {
	t.Helper()
	if _, managed := times[".github/dependabot.yml"]; managed {
		t.Fatal("protected destination gained a managed claim")
	}
	path := filepath.Join(root, ".github", "dependabot.yml")
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("protected destination written: %v", err)
	}
}

func TestInitUpdateOwnershipRetainsProtectedBaseline(t *testing.T) {
	t.Parallel()
	ta, repo, _, current := updatePristineFixture(t)
	repo.Write("CODEOWNERS", "* @core\n/.gitleaks.toml @platform\n")
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"init", "--update", "--apply", repo.Root}); code != exitClean {
		t.Fatal(ta.stderr.String())
	}
	got, err := os.ReadFile(filepath.Join(repo.Root, ".gitleaks.toml"))
	if err != nil || !bytes.Equal(got, []byte(current)) {
		t.Fatalf("protected managed file changed: %q, %v", got, err)
	}
	data, err := bootstrap.Current(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	config, err := policy.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, managed := range config.Managed {
		if managed.Path == ".gitleaks.toml" {
			found = managed.SHA256 == install.Digest([]byte(current))
		}
	}
	if !found || config.Kit.Version != testVersion {
		t.Fatalf("protected baseline or version lost: %+v", config)
	}
	out := ta.stdout.String()
	if !strings.Contains(out, ".gitleaks.toml") || strings.Contains(out, "+++ .gitleaks.toml") ||
		strings.Contains(out, "não pode gerar") || strings.Contains(out, "cannot render") {
		t.Fatalf("protected update report = %q", out)
	}
	requireSecondUpdate(t, ta, repo.Root)
}

func TestInitOwnershipDoesNotInspectProtectedTemplate(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Write("go.mod", "module example.test/service\n\ngo 1.27\n")
	repo.Write("CODEOWNERS", "* @core\n/.gitleaks.toml @platform\n")
	if err := os.Symlink(t.TempDir(), filepath.Join(repo.Root, ".gitleaks.toml")); err != nil {
		t.Fatal(err)
	}
	ta := initBindApp(t)
	if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("protected template inspection refused init: %s", ta.stderr.String())
	}
	if info, err := os.Lstat(filepath.Join(repo.Root, ".gitleaks.toml")); err != nil || info.Mode()&fs.ModeSymlink == 0 {
		t.Fatalf("protected link changed: %v, %v", info, err)
	}
}
