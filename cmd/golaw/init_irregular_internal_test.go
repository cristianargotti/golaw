package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/policy"
)

// irregularShape builds one destination init cannot write and answers the
// check that it was left as it was.
type irregularShape func(t *testing.T, root string) func(t *testing.T)

func TestInitPreservesLinkedAndIrregularDestinations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, preserved string
		shape           irregularShape
	}{
		{"linked file", ".editorconfig", linkedEditorConfig},
		{"linked directory", "docs/adr/0001-inherited-baseline.md", linkedDocs},
		{"file where a directory is needed", "tools/go.mod", regularTools},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			repo.Commit("chore: establish application", map[string]string{
				"go.mod": "module example.test/service\n\ngo 1.27\n",
			})
			unchanged := tc.shape(t, repo.Root)
			ta := initBindApp(t)
			if code := ta.run(t.Context(), []string{"init", "--apply", repo.Root}); code != exitClean {
				t.Fatalf("irregular destination stopped init: exit=%d: %s", code, ta.stderr.String())
			}
			unchanged(t)
			requireIrregularReport(t, ta, repo.Root, tc.preserved)
		})
	}
}

func linkedEditorConfig(t *testing.T, root string) func(t *testing.T) {
	t.Helper()
	const content = "root = true\n"
	writeIrregular(t, filepath.Join(root, "config", "editorconfig"), content)
	if err := os.Symlink(filepath.Join("config", "editorconfig"), filepath.Join(root, ".editorconfig")); err != nil {
		t.Fatal(err)
	}
	return func(t *testing.T) {
		t.Helper()
		requireLink(t, filepath.Join(root, ".editorconfig"))
		requireBytes(t, filepath.Join(root, "config", "editorconfig"), content)
	}
}

func linkedDocs(t *testing.T, root string) func(t *testing.T) {
	t.Helper()
	writeIrregular(t, filepath.Join(root, "manual", "README.md"), "manual\n")
	if err := os.Symlink("manual", filepath.Join(root, "docs")); err != nil {
		t.Fatal(err)
	}
	return func(t *testing.T) {
		t.Helper()
		requireLink(t, filepath.Join(root, "docs"))
		entries, err := os.ReadDir(filepath.Join(root, "manual"))
		if err != nil || len(entries) != 1 {
			t.Fatalf("linked directory written: %v, %v", entries, err)
		}
	}
}

func regularTools(t *testing.T, root string) func(t *testing.T) {
	t.Helper()
	const content = "tools are listed in the README\n"
	writeIrregular(t, filepath.Join(root, "tools"), content)
	return func(t *testing.T) {
		t.Helper()
		requireBytes(t, filepath.Join(root, "tools"), content)
	}
}

func writeIrregular(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func requireLink(t *testing.T, path string) {
	t.Helper()
	if info, err := os.Lstat(path); err != nil || info.Mode()&fs.ModeSymlink == 0 {
		t.Fatalf("link replaced: %s: %v, %v", path, info, err)
	}
}

func requireBytes(t *testing.T, path, want string) {
	t.Helper()
	if got, err := os.ReadFile(path); err != nil || string(got) != want {
		t.Fatalf("bytes changed: %s: %q, %v", path, got, err)
	}
}

// requireIrregularReport requires the preserved destination named in the
// human channel, the other templates written, and golaw.yml written and
// enrolled without a record of the preserved destination.
func requireIrregularReport(t *testing.T, ta testApp, root, preserved string) {
	t.Helper()
	line := "Destino com link simbólico ou caminho irregular, preservado: " + preserved + ".\n"
	if !strings.Contains(ta.stdout.String(), line) {
		t.Fatalf("irregular destination not named: %q", ta.stdout.String())
	}
	if _, err := os.Stat(filepath.Join(root, "lefthook.yml")); err != nil {
		t.Fatalf("allowed template missing: %v", err)
	}
	data, err := bootstrap.Current(root)
	if err != nil || data == nil {
		t.Fatalf("policy missing: %v", err)
	}
	config, err := policy.Decode(data)
	if err != nil || slices.ContainsFunc(config.Managed, func(m policy.ManagedFile) bool { return m.Path == preserved }) {
		t.Fatalf("preserved destination recorded: %+v, %v", config.Managed, err)
	}
	requireOwnershipEnrollment(t, ta, root, data, true)
}

func TestInitUpdatePreservesALinkedRecordedFile(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Write("go.mod", "module example.test/service\n\ngo 1.27\n")
	ta := initBindApp(t)
	olderPolicy(t, ta, repo, filepath.Join(repo.Root, bootstrap.PolicyFile))
	path := filepath.Join(repo.Root, ".gitleaks.toml")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	const outside = "title = \"outside\"\n"
	writeIrregular(t, filepath.Join(repo.Root, "shared", "gitleaks.toml"), outside)
	if err := os.Symlink(filepath.Join("shared", "gitleaks.toml"), path); err != nil {
		t.Fatal(err)
	}
	ta.stdout.Reset()
	if code := ta.run(t.Context(), []string{"init", "--update", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("linked recorded file stopped the update: exit=%d: %s", code, ta.stderr.String())
	}
	requireLink(t, path)
	requireBytes(t, filepath.Join(repo.Root, "shared", "gitleaks.toml"), outside)
	out := ta.stdout.String()
	if !strings.Contains(out, "Destino com link simbólico ou caminho irregular, preservado: .gitleaks.toml.\n") ||
		strings.Contains(out, "não pode gerar") {
		t.Fatalf("linked recorded file report = %q", out)
	}
	data, err := bootstrap.Current(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	config, err := policy.Decode(data)
	if err != nil || config.Kit.Version != testVersion ||
		!slices.ContainsFunc(config.Managed, func(m policy.ManagedFile) bool { return m.Path == ".gitleaks.toml" }) {
		t.Fatalf("linked recorded file lost its record or the pin: %+v, %v", config, err)
	}
}
