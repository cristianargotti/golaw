package policyfiles_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

const foreignRules = "* @app\n/private/ @platform\n"

func TestCodeownersForeignBasePrecedence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		files        map[string][]byte
		changes      []model.Change
	}{
		{
			name: "existing highest priority", source: ".github/CODEOWNERS",
			files: map[string][]byte{
				".github/CODEOWNERS": []byte(foreignRules), "CODEOWNERS": []byte("* @app\n"),
				"docs/CODEOWNERS": []byte("* @app\n"),
			},
		},
		{
			name: "added highest priority cannot displace root", source: "CODEOWNERS",
			files: map[string][]byte{".github/CODEOWNERS": []byte("* @app\n"), "CODEOWNERS": []byte(foreignRules)},
			changes: []model.Change{{
				Op: model.ChangeAdd, NewPath: ".github/CODEOWNERS", After: "candidate", Mode: "100644",
			}},
		},
		{
			name: "renamed in highest priority cannot displace root", source: "CODEOWNERS",
			files: map[string][]byte{".github/CODEOWNERS": []byte("* @app\n"), "CODEOWNERS": []byte(foreignRules)},
			changes: []model.Change{{
				Op: model.ChangeRename, OldPath: "owners.txt", NewPath: ".github/CODEOWNERS",
				Before: "candidate", After: "candidate", Mode: "100644",
			}},
		},
		{
			name: "renamed away highest priority retains protection", source: ".github/CODEOWNERS",
			files: map[string][]byte{"owners.txt": []byte("* @app\n"), "CODEOWNERS": []byte("* @app\n")},
			changes: []model.Change{{
				Op: model.ChangeRename, OldPath: ".github/CODEOWNERS", NewPath: "owners.txt",
				Before: "base", After: "candidate", Mode: "100644",
			}},
		},
		{
			name: "both higher priorities added", source: "docs/CODEOWNERS",
			files: map[string][]byte{
				".github/CODEOWNERS": []byte("* @app\n"), "CODEOWNERS": []byte("* @app\n"),
				"docs/CODEOWNERS": []byte(foreignRules),
			},
			changes: []model.Change{
				{Op: model.ChangeAdd, NewPath: ".github/CODEOWNERS", Mode: "100644"},
				{Op: model.ChangeAdd, NewPath: "CODEOWNERS", Mode: "100644"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			changes := append(slices.Clone(tc.changes), model.Change{Op: model.ChangeAdd, NewPath: "private/key.txt"})
			tree := proof.NewBlobTree(tc.files, map[string][]byte{"base": []byte(foreignRules)})
			result := runForeign(t, tree, changes)
			requireForeignFinding(t, result, tc.source, "private/key.txt")
		})
	}
}

func TestCodeownersForeignReplacementIsOrderIndependent(t *testing.T) {
	t.Parallel()
	for _, renamed := range []bool{false, true} {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("rename=%t/reverse=%t", renamed, reverse), func(t *testing.T) {
				t.Parallel()
				removed := model.Change{Op: model.ChangeDelete, OldPath: ".github/CODEOWNERS", Before: "base"}
				added := model.Change{Op: model.ChangeAdd, NewPath: ".github/CODEOWNERS", After: "candidate"}
				if renamed {
					removed.Op, removed.NewPath, removed.After = model.ChangeRename, "retired.txt", "base"
					added.Op, added.OldPath, added.Before = model.ChangeRename, "candidate.txt", "candidate"
				}
				changes := make([]model.Change, 0, 3)
				changes = append(changes, removed, added)
				if reverse {
					slices.Reverse(changes)
				}
				changes = append(changes, model.Change{Op: model.ChangeAdd, NewPath: "private/key.txt"})
				tree := proof.NewBlobTree(map[string][]byte{".github/CODEOWNERS": []byte("* @app\n")},
					map[string][]byte{"base": []byte(foreignRules), "candidate": []byte("* @app\n")})
				requireForeignFinding(t, runForeign(t, tree, changes), ".github/CODEOWNERS", "private/key.txt")
			})
		}
	}
}

func TestCodeownersForeignDeduplicatesBothRenameEndpoints(t *testing.T) {
	t.Parallel()
	tree := proof.NewBlobTree(map[string][]byte{"CODEOWNERS": []byte(foreignRules)}, nil)
	changes := []model.Change{
		{Op: model.ChangeRename, OldPath: "private/old.txt", NewPath: "private/new.txt"},
		{Op: model.ChangeModify, OldPath: "private/new.txt", NewPath: "private/new.txt"},
		{Op: model.ChangeDelete, OldPath: "private/old.txt"},
	}
	result := runForeign(t, tree, changes)
	if result.Status.Kind != model.StatusMeasured || len(result.Findings) != 2 {
		t.Fatalf("result = %+v", result)
	}
	for _, path := range []string{"private/old.txt", "private/new.txt"} {
		count := 0
		for _, item := range result.Findings {
			if strings.Contains(item.Message, path) {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("findings for %s = %d: %+v", path, count, result)
		}
	}
}

func runForeign(t *testing.T, snapshot model.Snapshot, changes []model.Change) model.Result {
	t.Helper()
	result, err := policyfiles.CodeownersForeign().Run(t.Context(), model.Input{
		Mode: model.ModeStaged, Policy: proof.PolicyFor(proof.Entry{Profile: "fixture"}),
		Snapshot: snapshot, Changes: changes,
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func requireForeignFinding(t *testing.T, result model.Result, source, changed string) {
	t.Helper()
	if result.Status.Kind != model.StatusMeasured || len(result.Findings) != 1 {
		t.Fatalf("result = %+v", result)
	}
	item := result.Findings[0]
	if item.Path != source || item.Line != 2 || item.Code != "foreign-owned-change" || item.Severity != model.SeverityDeny {
		t.Fatalf("finding = %+v", item)
	}
	for _, text := range []string{changed, "@platform", source, "owning team", "change this path", "pre_commit: false"} {
		if !strings.Contains(item.Message, text) {
			t.Fatalf("message %q does not contain %q", item.Message, text)
		}
	}
}
