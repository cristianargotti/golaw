package bootstrap_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestHoldRecordsKeepsThePolicyRecordsAndProposesNewKitFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	recorded, stale := install.Digest([]byte("recorded\n")), install.Digest([]byte("stale\n"))
	config := fmt.Sprintf("golaw: 1\nkit: {version: 0.1.0}\nmanaged:\n"+
		"  - {path: kept.txt, sha256: %s}\n  - {path: refreshed.txt, sha256: %s}\n", recorded, stale)
	writeBefore(t, filepath.Join(root, bootstrap.PolicyFile), config)
	writeBefore(t, filepath.Join(root, "kept.txt"), "recorded\n")
	writeBefore(t, filepath.Join(root, "refreshed.txt"), "fresh\n")
	writeBefore(t, filepath.Join(root, "matching.txt"), "matching\n")
	plan, err := bootstrap.PlanTemplates(root, []assets.Rendered{
		templateFile("kept.txt", "recorded\n", assets.OwnedByKit),
		templateFile("refreshed.txt", "fresh\n", assets.OwnedByKit),
		templateFile("matching.txt", "matching\n", assets.OwnedByKit),
		templateFile("new.txt", "new\n", assets.OwnedByKit),
		templateFile("notes.md", "notes\n", assets.OwnedByDeveloper),
	})
	if err != nil {
		t.Fatal(err)
	}
	plan.HoldRecords()
	held := plan
	wantHeld := []policy.ManagedFile{
		{Path: "matching.txt", SHA256: install.Digest([]byte("matching\n"))},
		{Path: "new.txt", SHA256: install.Digest([]byte("new\n"))},
		{Path: "refreshed.txt", SHA256: install.Digest([]byte("fresh\n"))},
	}
	wantManaged := []policy.ManagedFile{{Path: "kept.txt", SHA256: recorded}, {Path: "refreshed.txt", SHA256: stale}}
	if !reflect.DeepEqual(held.Held, wantHeld) || !reflect.DeepEqual(held.Managed, wantManaged) {
		t.Fatalf("held = %+v, managed = %+v", held.Held, held.Managed)
	}
	if len(held.Changes) != 1 || held.Changes[0].Path != "notes.md" ||
		len(held.Proposed) != 1 || held.Proposed[0].Path != "new.txt" {
		t.Fatalf("held writes = %+v, proposals = %+v", held.Changes, held.Proposed)
	}
	var out bytes.Buffer
	if applyErr := bootstrap.ApplyTemplates(root, held, &out); applyErr != nil {
		t.Fatal(applyErr)
	}
	requireTemplateAbsent(t, root, "new.txt")
	if !strings.Contains(out.String(), "Propostas sem gravação:\n--- new.txt\n+++ new.txt\n+new\n") {
		t.Fatalf("held file not proposed: %q", out.String())
	}
}

func TestTemplateDiffReportsHeldRecords(t *testing.T) {
	t.Parallel()
	hash := install.Digest([]byte("new\n"))
	for _, tc := range []struct{ language, line string }{
		{"pt-BR", "Template sem registro, CODEOWNERS protege golaw.yml: new.txt sha256=" + hash +
			"; um responsável por golaw.yml registra esse hash em managed e faz o commit do arquivo.\n"},
		{"en", "Template without a record, CODEOWNERS protects golaw.yml: new.txt sha256=" + hash +
			"; an owner of golaw.yml records this hash under managed and commits the file.\n"},
	} {
		t.Run(tc.language, func(t *testing.T) {
			t.Parallel()
			plan := bootstrap.TemplatePlan{
				Language: tc.language, Held: []policy.ManagedFile{{Path: "new.txt", SHA256: hash}},
			}
			var out bytes.Buffer
			if err := bootstrap.WriteTemplateDiff(&out, plan); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.line {
				t.Fatalf("held report = %q, want %q", out.String(), tc.line)
			}
		})
	}
}
