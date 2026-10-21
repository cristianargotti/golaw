package bootstrap_test

import (
	"io"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
)

// TestUpdateWritesTheKitTemplatesANewVersionAdds pins the update of a
// repository adopted under an earlier kit version: the kit file the new
// version adds is written and recorded beside the managed file that names
// it, a kit destination the tree already holds is only proposed, a merged
// file the tree holds gets its missing lines proposed and a developer file
// stays absent.
func TestUpdateWritesTheKitTemplatesANewVersionAdds(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCurrent(t, filepath.Join(root, ".golangci.yml"), "old\n")
	writeCurrent(t, filepath.Join(root, "held.yml"), "the person's own file\n")
	writeCurrent(t, filepath.Join(root, "go.mod"), "module example.test/app\n")
	recorded := []policy.ManagedFile{{Path: ".golangci.yml", SHA256: install.Digest([]byte("old\n"))}}
	plan, err := bootstrap.PlanUpdate(root, []assets.Rendered{
		templateFile(".golangci.yml", "rules: scripts/lint/rules.go\n", assets.OwnedByKit),
		templateFile("held.yml", "kit bytes\n", assets.OwnedByKit),
		templateFile("go.mod", "require example.test/dsl v1.0.0\n", assets.OwnedMerged),
		templateFile("Makefile", "include golaw.mk\n", assets.OwnedMerged),
		templateFile("notes.md", "notes\n", assets.OwnedByDeveloper),
		templateFile("scripts/lint/rules.go", "package gorules\n", assets.OwnedByKit),
	}, recorded)
	if err != nil {
		t.Fatal(err)
	}
	written := make([]string, 0, len(plan.Changes))
	for _, change := range plan.Changes {
		written = append(written, change.Path)
	}
	proposed := make([]string, 0, len(plan.Proposed))
	for _, change := range plan.Proposed {
		proposed = append(proposed, change.Path)
	}
	managed := []policy.ManagedFile{
		{Path: ".golangci.yml", SHA256: install.Digest([]byte("rules: scripts/lint/rules.go\n"))},
		{Path: "scripts/lint/rules.go", SHA256: install.Digest([]byte("package gorules\n"))},
	}
	if !slices.Equal(written, []string{".golangci.yml", "scripts/lint/rules.go"}) ||
		!slices.Equal(proposed, []string{"go.mod", "held.yml"}) ||
		!slices.Equal(plan.Managed, managed) || len(plan.LocalEdits) != 0 {
		t.Fatalf("plan = written %v, proposed %v, managed %+v, local edits %+v",
			written, proposed, plan.Managed, plan.LocalEdits)
	}
	if err := bootstrap.ApplyTemplates(root, plan, io.Discard); err != nil {
		t.Fatal(err)
	}
	requireUpdatedBytes(t, filepath.Join(root, "scripts", "lint", "rules.go"), "package gorules\n")
	requireUpdatedBytes(t, filepath.Join(root, "held.yml"), "the person's own file\n")
	requireUpdatedBytes(t, filepath.Join(root, "go.mod"), "module example.test/app\n")
	requireTemplateAbsent(t, root, "Makefile")
	requireTemplateAbsent(t, root, "notes.md")
}
