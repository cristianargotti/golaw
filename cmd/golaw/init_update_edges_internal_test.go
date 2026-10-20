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

func TestInitUpdateKeepsReviewedSettingsAndUnrenderableRecords(t *testing.T) {
	t.Parallel()
	for _, directive := range []string{"", "1.23", "1.27"} {
		t.Run("go "+directive, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			if directive != "" {
				repo.Write("go.mod", "module example.test/service\n\ngo "+directive+"\n")
			}
			ta := initBindApp(t)
			hash := install.Digest([]byte("retired template\n"))
			reviewed := "golaw: 1\nkit: { version: 9.9.8 } # reviewed pin\nlanguage: en\n" +
				"checks: {sizes: {file_lines: 200}}\nmanaged:\n" +
				"  - {path: retired.conf, sha256: " + hash + "}\n"
			repo.Write(bootstrap.PolicyFile, reviewed)
			args := []string{"init", "--update", "--apply", "--language", "pt-BR", "--capabilities", "unknown", repo.Root}
			if code := ta.run(t.Context(), args); code != exitClean {
				t.Fatalf("update exit = %d: %s", code, ta.stderr.String())
			}
			requireUnrenderableUpdate(t, ta, repo.Root, reviewed, hash)
			requireNoInitTemplates(t, repo.Root)
			requireUnarmed(t, repo.GitDir())
		})
	}
}

func requireUnrenderableUpdate(t *testing.T, ta testApp, root, reviewed, hash string) {
	t.Helper()
	want := strings.Replace(reviewed, "9.9.8", testVersion, 1)
	got, err := bootstrap.Current(root)
	if err != nil || !bytes.Equal(got, []byte(want)) {
		t.Fatalf("reviewed bytes = %q, want %q: %v", got, want, err)
	}
	out := ta.stdout.String()
	if !strings.Contains(out, "cannot render") || !strings.Contains(out, "retired.conf") ||
		!strings.Contains(out, hash) || !strings.HasSuffix(out, commitLine("en")+"\n") {
		t.Fatalf("unknown record report = %q", out)
	}
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	record, found, err := policy.ReadRecord(dirs.State, root)
	if err != nil || !found || record.Bootstrap != want || record.KitVersion != testVersion {
		t.Fatalf("updated enrollment = %+v, %v, %v", record, found, err)
	}
}

func TestInitUpdateWritesOnlyPristineRecordedFiles(t *testing.T) {
	t.Parallel()
	ta, repo, original, current := updatePristineFixture(t)
	path := filepath.Join(repo.Root, ".gitleaks.toml")
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"init", "--update", repo.Root}); code != exitClean {
		t.Fatalf("preview exit = %d: %s", code, ta.stderr.String())
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != current {
		t.Fatalf("preview wrote template = %q, %v", got, err)
	}
	if !strings.Contains(ta.stdout.String(), "+++ .gitleaks.toml\n") {
		t.Fatalf("missing pristine diff: %q", ta.stdout.String())
	}
	relation := "Atualização gerenciada: .gitleaks.toml registrado=" + install.Digest([]byte(current)) +
		" atual=" + install.Digest([]byte(current)) + " proposto=" + install.Digest(original) + "."
	if !strings.Contains(ta.stdout.String(), relation) {
		t.Fatalf("missing recorded-hash relationship: %q", ta.stdout.String())
	}
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"init", "--update", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("apply exit = %d: %s", code, ta.stderr.String())
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("pristine template = %q, %v", got, err)
	}
	data, err := bootstrap.Current(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	config, err := policy.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range config.Managed {
		if record.Path == ".gitleaks.toml" && record.SHA256 != install.Digest(original) {
			t.Fatalf("new baseline = %q", record.SHA256)
		}
	}
	requireSecondUpdate(t, ta, repo.Root)
	if strings.Contains(ta.stdout.String(), "Atualização gerenciada:") {
		t.Fatalf("unchanged update reported a write: %q", ta.stdout.String())
	}
	requireUnarmed(t, repo.GitDir())
}

func updatePristineFixture(t *testing.T) (testApp, *gittest.Repo, []byte, string) {
	t.Helper()
	repo := gittest.New(t)
	repo.Write("go.mod", "module example.test/service\n\ngo 1.27\n")
	ta := initBindApp(t)
	older, _ := olderPolicy(t, ta, repo, filepath.Join(repo.Root, bootstrap.PolicyFile))
	original, err := os.ReadFile(filepath.Join(repo.Root, ".gitleaks.toml"))
	if err != nil {
		t.Fatal(err)
	}
	const current = "title = \"previous template\"\n"
	repo.Write(".gitleaks.toml", current)
	repo.Write(bootstrap.PolicyFile, strings.Replace(older, install.Digest(original), install.Digest([]byte(current)), 1))
	return ta, repo, original, current
}

func TestInitUpdateReportsDeletionWithoutRecreatingIt(t *testing.T) {
	t.Parallel()
	ta, repo, _, current := updatePristineFixture(t)
	path := filepath.Join(repo.Root, ".gitleaks.toml")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"init", "--update", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("update exit = %d: %s", code, ta.stderr.String())
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("deleted template recreated: %v", err)
	}
	out := ta.stdout.String()
	for _, want := range []string{".gitleaks.toml", install.Digest([]byte(current)), "atual=ausente"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in deletion report: %q", want, out)
		}
	}
	if strings.Contains(out, "+++ .gitleaks.toml") {
		t.Fatalf("local deletion printed as a write diff: %q", out)
	}
	requireSecondUpdate(t, ta, repo.Root)
}

// Plain init keeps a managed file the team deleted deleted, as the update
// does, and names it with the same line, in the preview and on --apply.
func TestInitKeepsADeletedManagedFileDeleted(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Write("go.mod", "module example.test/service\n\ngo 1.27\n")
	ta := initBindApp(t)
	if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitClean {
		t.Fatal(ta.stderr.String())
	}
	path := filepath.Join(repo.Root, ".gremlins.yaml")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	policyBefore, err := bootstrap.Current(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	repo.Remove(".gremlins.yaml")
	hash := install.Digest(original)
	line := "Edição local preservada: .gremlins.yaml registrado=" + hash + " atual=ausente proposto=" + hash + ".\n"
	for _, args := range [][]string{{"init", repo.Root}, {"init", "--apply", repo.Root}} {
		ta.stdout.Reset()
		if code := ta.run(t.Context(), args); code != exitClean {
			t.Fatalf("%v exit = %d: %s", args, code, ta.stderr.String())
		}
		if !strings.Contains(ta.stdout.String(), line) || strings.Contains(ta.stdout.String(), "+++ .gremlins.yaml") {
			t.Fatalf("%v deletion report = %q", args, ta.stdout.String())
		}
		if _, statErr := os.Stat(path); !errors.Is(statErr, fs.ErrNotExist) {
			t.Fatalf("%v recreated the deleted file: %v", args, statErr)
		}
	}
	if after, readErr := bootstrap.Current(repo.Root); readErr != nil || !bytes.Equal(after, policyBefore) {
		t.Fatalf("deletion changed the policy: %q, %v", after, readErr)
	}
}
