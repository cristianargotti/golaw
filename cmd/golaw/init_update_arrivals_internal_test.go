package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// TestUpdateNamesTheKitFilesANewVersionAdds pins the line init --update
// prints for a kit file the new version adds, apart from the line of a
// managed file it rewrites, in both languages of the human channel.
func TestUpdateNamesTheKitFilesANewVersionAdds(t *testing.T) {
	t.Parallel()
	rules, config := []byte("package gorules\n"), []byte("new\n")
	plan := bootstrap.TemplatePlan{Changes: []install.Change{
		{Path: ".golangci.yml", Before: []byte("old\n"), After: config, Existed: true},
		{Path: "scripts/lint/rules.go", After: rules},
	}}
	old := install.Digest([]byte("old\n"))
	for _, tc := range []struct {
		language string
		want     []string
	}{
		{model.DefaultLanguage, []string{
			"Atualização gerenciada: .golangci.yml registrado=" + old + " atual=" + old + " proposto=" + install.Digest(config) + ".",
			"Arquivo novo do kit: scripts/lint/rules.go proposto=" + install.Digest(rules) + ".",
		}},
		{"en", []string{
			"Managed update: .golangci.yml recorded=" + old + " current=" + old + " proposed=" + install.Digest(config) + ".",
			"New kit file: scripts/lint/rules.go proposed=" + install.Digest(rules) + ".",
		}},
	} {
		plan.Language = tc.language
		if got := updateWriteNotices(plan); !slices.Equal(got, tc.want) {
			t.Fatalf("%s: notices = %q, want %q", tc.language, got, tc.want)
		}
	}
}

func TestInitUpdatePreservesIrregularArrivals(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, preserved string
		shape           irregularShape
	}{
		{"merged link", ".editorconfig", linkedEditorConfig},
		{"kit parent file", "tools/go.mod", regularTools},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			repo.Write("go.mod", "module example.test/service\n\ngo 1.27\n")
			repo.Write(".golangci.yml", "old\n")
			repo.Write(bootstrap.PolicyFile, "golaw: 1\nkit: {version: 9.9.8}\nmanaged:\n"+
				"  - {path: .golangci.yml, sha256: "+install.Digest([]byte("old\n"))+"}\n")
			unchanged := tc.shape(t, repo.Root)
			ta := initBindApp(t)
			if code := ta.run(t.Context(), []string{"init", "--update", "--apply", repo.Root}); code != exitClean {
				t.Fatalf("irregular arrival stopped update: exit=%d: %s", code, ta.stderr.String())
			}
			unchanged(t)
			requireIrregularReport(t, ta, repo.Root, tc.preserved)
			requireTemplatePresence(t, repo.Root, ".gitleaks.toml", true)
			requireUnarmed(t, repo.GitDir())
		})
	}
}

func TestInitUpdateDoesNotInspectUnadoptedArrivals(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Write("go.mod", "module example.test/service\n\ngo 1.27\n")
	repo.Write(bootstrap.PolicyFile, "golaw: 1\nkit: {version: 9.9.8}\n")
	unchanged := regularTools(t, repo.Root)
	ta := initBindApp(t)
	if code := ta.run(t.Context(), []string{"init", "--update", "--apply", repo.Root}); code != exitClean {
		t.Fatalf("unadopted update failed: exit=%d: %s", code, ta.stderr.String())
	}
	unchanged(t)
	requireNoInitTemplates(t, repo.Root)
	if strings.Contains(ta.stdout.String(), "caminho irregular") {
		t.Fatalf("unadopted arrival was inspected: %s", ta.stdout.String())
	}
}

func TestInitUpdateDoesNotAdoptThroughAFilteredRecord(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, owners string }{
		{"irregular", "* @core\n"},
		{"protected", "* @core\n/tools/ @platform\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			repo.Write("go.mod", "module example.test/service\n\ngo 1.27\n")
			hash := install.Digest([]byte("old\n"))
			repo.Write(bootstrap.PolicyFile, "golaw: 1\nkit: {version: 9.9.8}\nmanaged:\n"+
				"  - {path: tools/go.mod, sha256: "+hash+"}\n")
			repo.Write("CODEOWNERS", tc.owners)
			unchangedRecord := regularTools(t, repo.Root)
			unchangedArrival := linkedEditorConfig(t, repo.Root)
			ta := initBindApp(t)
			if code := ta.run(t.Context(), []string{"init", "--update", "--apply", repo.Root}); code != exitClean {
				t.Fatalf("filtered record update failed: exit=%d: %s", code, ta.stderr.String())
			}
			unchangedRecord(t)
			unchangedArrival(t)
			requireTemplatePresence(t, repo.Root, "lefthook.yml", false)
			requireTemplatePresence(t, repo.Root, ".gitleaks.toml", false)
			if strings.Contains(ta.stdout.String(), ".editorconfig") {
				t.Fatalf("filtered record enabled arrival inspection: %s", ta.stdout.String())
			}
			data, err := bootstrap.Current(repo.Root)
			if err != nil {
				t.Fatal(err)
			}
			config, err := policy.Decode(data)
			want := []policy.ManagedFile{{Path: "tools/go.mod", SHA256: hash}}
			if err != nil || !slices.Equal(config.Managed, want) {
				t.Fatalf("filtered record changed: %+v, %v", config.Managed, err)
			}
		})
	}
}
