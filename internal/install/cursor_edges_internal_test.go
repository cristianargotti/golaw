package install

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
)

func TestCursorPlanInputBoundaries(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		file string
		want error
	}{
		{"red incompatible hooks", "testdata/cursor_invalid.json", ErrHooks},
		{"near foreign hooks", "testdata/cursor_foreign.json", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			options := cursorOptions(t, "user")
			path := filepath.Join(options.ConfigDir, "hooks.json")
			data := cursorRead(t, test.file)
			must(t, WriteAtomic(path, data))
			_, err := PlanCursor(options)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if !bytes.Equal(cursorRead(t, path), data) {
				t.Fatal("planning changed input")
			}
		})
	}
}

func TestCursorPlanSharedSkill(t *testing.T) {
	t.Parallel()
	for _, changed := range []bool{true, false} {
		t.Run("skill", func(t *testing.T) {
			t.Parallel()
			options := cursorOptions(t, "user")
			data, err := assets.Read("agents/SKILL.md")
			must(t, err)
			if changed {
				data = []byte("personal instructions\n")
			}
			must(t, WriteAtomic(SkillPath(options), data))
			_, err = PlanCursor(options)
			if changed && !errors.Is(err, ErrChanged) {
				t.Fatalf("changed skill error = %v", err)
			}
			if !changed && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCursorPlanUpgradeAndStaleApply(t *testing.T) {
	t.Parallel()
	options, first := cursorInstalled(t, "project")
	options.Version = "0.4.1"
	options.Executable = filepath.Join(options.Home, "new bin", "golaw")
	upgrade, err := PlanCursor(options)
	must(t, err)
	if len(upgrade.Receipt.Entries) != len(first.Receipt.Entries) {
		t.Fatal("upgrade duplicated handlers")
	}
	for index, entry := range upgrade.Receipt.Entries {
		if entry.Group != first.Receipt.Entries[index].Group {
			t.Fatal("upgrade moved an owned handler")
		}
	}
	path := first.Receipt.HooksPath
	before := cursorRead(t, path)
	must(t, WriteAtomic(path, append(bytes.Clone(before), '\n')))
	err = Apply(t.Context(), upgrade, &bytes.Buffer{}, &cursorEnrollment{})
	if !errors.Is(err, ErrChanged) {
		t.Fatalf("stale plan error = %v", err)
	}
	must(t, WriteAtomic(path, before))
	cursorApply(t, upgrade)
	repeated, err := PlanCursor(options)
	must(t, err)
	if len(repeated.Changes) != 0 {
		t.Fatal("upgraded install is not idempotent")
	}
	older := []byte("An older golaw skill.\n")
	must(t, WriteAtomic(SkillPath(options), older))
	receipt, err := ReadReceipt(upgrade.ReceiptPath)
	must(t, err)
	for index := range receipt.Artifacts {
		if receipt.Artifacts[index].Path == SkillPath(options) {
			receipt.Artifacts[index].SHA256 = Digest(older)
		}
	}
	cursorWriteReceipt(t, upgrade.ReceiptPath, receipt)
	skilled, err := PlanCursor(options)
	must(t, err)
	if len(skilled.Changes) != 2 || skilled.Changes[0].Path != SkillPath(options) || skilled.Changes[1].Path != skilled.ReceiptPath {
		t.Fatalf("an older skill the receipt recorded = %+v", skilled.Changes)
	}
}
