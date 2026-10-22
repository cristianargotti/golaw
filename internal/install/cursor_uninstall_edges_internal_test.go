package install

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCursorUninstallRefusesEvidenceItDidNotWrite(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		edit func(receipt *Receipt)
	}{
		{"unknown scope", func(receipt *Receipt) { receipt.Scope = "global" }},
		{"scope of another folder", func(receipt *Receipt) { receipt.Scope = "user" }},
		{"bundle elsewhere", func(receipt *Receipt) { receipt.Bundle = filepath.Dir(receipt.Bundle) }},
		{"hooks outside a cursor folder", func(receipt *Receipt) {
			receipt.HooksPath = filepath.Join(filepath.Dir(filepath.Dir(receipt.HooksPath)), "hooks.json")
		}},
		{"artifact no install writes", func(receipt *Receipt) {
			receipt.Artifacts = append(receipt.Artifacts, Artifact{Path: filepath.Join(receipt.Bundle, "notes.md")})
		}},
		{"skill of another project", func(receipt *Receipt) {
			other := filepath.Join(filepath.Dir(receipt.Bundle), "other", ".agents", "skills", "golaw", "SKILL.md")
			receipt.Artifacts = append(receipt.Artifacts, Artifact{Path: other})
		}},
		{"entry of another tool", func(receipt *Receipt) {
			receipt.Entries[0].Definition = json.RawMessage(`{"command":"first-check","matcher":"Write"}`)
		}},
		{"entry of an event golaw never installs", func(receipt *Receipt) { receipt.Entries[0].Event = "stop" }},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			_, plan := cursorInstalled(t, "project")
			receipt, err := ReadReceipt(plan.ReceiptPath)
			must(t, err)
			row.edit(&receipt)
			cursorWriteReceipt(t, plan.ReceiptPath, receipt)
			if _, err := UninstallCursor(plan.ReceiptPath); !errors.Is(err, ErrPath) {
				t.Fatalf("error = %v, want %v", err, ErrPath)
			}
		})
	}
}

func TestCursorUninstallOfWhatIsAlreadyGone(t *testing.T) {
	t.Parallel()
	options, plan := cursorInstalled(t, "project")
	for _, path := range []string{
		plan.Receipt.HooksPath, SkillPath(options),
		filepath.Join(options.Root, ".cursor", "rules", "golaw.mdc"),
	} {
		must(t, os.Remove(path))
	}
	uninstall, err := UninstallCursor(plan.ReceiptPath)
	must(t, err)
	if len(uninstall.Changes) != 1 || uninstall.Changes[0].Path != plan.ReceiptPath || !uninstall.Changes[0].Remove {
		t.Fatalf("changes = %+v", uninstall.Changes)
	}
	if _, err := UninstallCursor(filepath.Join(options.StateDir, "cursor", "user", "receipt.json")); err == nil {
		t.Fatal("an absent receipt planned a removal")
	}
}

func TestCursorUninstallKeepsWhatOthersLeft(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		edit func(handlers map[string][]json.RawMessage)
		kept string
	}{
		{"an event another tool left empty", func(handlers map[string][]json.RawMessage) {
			handlers["stop"] = []json.RawMessage{}
		}, "stop"},
		{"a handler the person removed", func(handlers map[string][]json.RawMessage) {
			delete(handlers, "postToolUse")
			handlers["sessionStart"] = []json.RawMessage{json.RawMessage(`{"command":"other"}`)}
		}, "sessionStart"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			_, plan := cursorInstalled(t, "user")
			handlers := cursorCompactHandlers(t, plan.Receipt.HooksPath)
			row.edit(handlers)
			cursorRewrite(t, plan.Receipt.HooksPath, handlers)
			uninstall, err := UninstallCursor(plan.ReceiptPath)
			must(t, err)
			cursorApply(t, uninstall)
			remaining := cursorHandlers(t, plan.Receipt.HooksPath)
			if _, found := remaining[row.kept]; !found || len(remaining) != 1 {
				t.Fatalf("remaining = %#v", remaining)
			}
		})
	}
}

func TestCursorUninstallRefusesALinkOnTheWay(t *testing.T) {
	t.Parallel()
	options, plan := cursorInstalled(t, "user")
	target := filepath.Join(options.Home, "elsewhere")
	must(t, os.Rename(filepath.Dir(plan.Receipt.HooksPath), target))
	if err := os.Symlink(target, filepath.Dir(plan.Receipt.HooksPath)); err != nil {
		t.Skipf("no symbolic link can be made here: %v", err)
	}
	if _, err := UninstallCursor(plan.ReceiptPath); !errors.Is(err, ErrPath) {
		t.Fatalf("error = %v, want %v", err, ErrPath)
	}
}
