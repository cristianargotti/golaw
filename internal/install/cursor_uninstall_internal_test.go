package install

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestCursorUninstallSharedOwnership(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		owner bool
		keep  bool
	}{
		{"red another owner", true, true},
		{"near sole owner", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cursorUninstallShared(t, test.owner, test.keep)
		})
	}
}

func cursorUninstallShared(t *testing.T, owner, keep bool) {
	t.Helper()
	options := cursorOptions(t, "project")
	hooksPath := filepath.Join(options.Root, ".cursor", "hooks.json")
	must(t, WriteAtomic(hooksPath, cursorRead(t, "testdata/cursor_foreign.json")))
	plan, err := PlanCursor(options)
	must(t, err)
	enrollment := cursorApply(t, plan)
	cursorCheckEnrollment(t, enrollment, options, 1)
	if owner {
		cursorAddSkillOwner(t, options, plan)
	}
	uninstall, err := UninstallCursor(plan.ReceiptPath)
	must(t, err)
	if !uninstall.Uninstall {
		t.Fatal("uninstall would enroll")
	}
	enrollment = cursorApply(t, uninstall)
	cursorCheckEnrollment(t, enrollment, options, 0)
	handlers := cursorHandlers(t, hooksPath)
	if len(handlers["preToolUse"]) != 2 || len(handlers["beforeReadFile"]) != 1 {
		t.Fatalf("foreign handlers changed: %#v", handlers)
	}
	cursorCheckSkillRemoval(t, SkillPath(options), keep)
	cursorAssertAbsent(t, plan.ReceiptPath)
	cursorAssertAbsent(t, filepath.Join(options.Root, ".cursor", "rules", "golaw.mdc"))
}

func cursorAddSkillOwner(t *testing.T, options Options, plan Plan) {
	t.Helper()
	skill := SkillPath(options)
	path := filepath.Join(options.StateDir, "codex", "project",
		plan.Receipt.ProjectID, "receipt.json")
	other := Receipt{
		Schema: 1, Agent: "codex", Scope: "project",
		ProjectID: plan.Receipt.ProjectID, Version: options.Version,
		Bundle:    filepath.Dir(path),
		Artifacts: []Artifact{{Path: skill, SHA256: Digest(cursorRead(t, skill))}},
	}
	cursorWriteReceipt(t, path, other)
}

func cursorCheckSkillRemoval(t *testing.T, skill string, keep bool) {
	t.Helper()
	_, err := os.Stat(skill)
	if keep {
		must(t, err)
		return
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("sole skill remains: %v", err)
	}
}

func cursorAssertAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("owned file remains: %s: %v", path, err)
	}
}

func TestCursorUninstallChangedArtifact(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		edit bool
	}{
		{"red changed rule", true},
		{"near unchanged rule", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cursorUninstallArtifact(t, test.edit)
		})
	}
}

func cursorUninstallArtifact(t *testing.T, edit bool) {
	t.Helper()
	options, plan := cursorInstalled(t, "project")
	path := filepath.Join(options.Root, ".cursor", "rules", "golaw.mdc")
	if edit {
		must(t, WriteAtomic(path, []byte("personal rule\n")))
	}
	_, err := UninstallCursor(plan.ReceiptPath)
	if edit && !errors.Is(err, ErrChanged) {
		t.Fatalf("changed artifact error = %v", err)
	}
	if !edit && err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(plan.ReceiptPath); statErr != nil {
		t.Fatalf("planning removed receipt: %v", statErr)
	}
}
