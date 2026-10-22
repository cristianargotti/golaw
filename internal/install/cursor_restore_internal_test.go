package install

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestCursorUninstallMovedAndChangedHandlers(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		mode string
		bad  bool
	}{
		{"red changed definition", "changed", true},
		{"red ambiguous copies", "duplicate", true},
		{"near moved definition", "moved", false},
		{"near unchanged definition", "unchanged", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cursorUninstallHandlers(t, test.mode, test.bad)
		})
	}
}

func cursorUninstallHandlers(t *testing.T, mode string, bad bool) {
	t.Helper()
	options, plan := cursorInstalled(t, "user")
	cursorChangeHandlers(t, plan, mode)
	uninstall, err := UninstallCursor(plan.ReceiptPath)
	if bad {
		if !errors.Is(err, ErrChanged) {
			t.Fatalf("ownership error = %v", err)
		}
		return
	}
	must(t, err)
	enrollment := cursorApply(t, uninstall)
	cursorCheckEnrollment(t, enrollment, options, 0)
	if mode == "unchanged" {
		// The install created the hooks file and nothing foreign is in it.
		cursorAssertAbsent(t, plan.Receipt.HooksPath)
		return
	}
	remaining := cursorHandlers(t, plan.Receipt.HooksPath)
	if len(remaining["preToolUse"]) != 1 {
		t.Fatalf("remaining handlers = %#v", remaining)
	}
	notes := strings.Join(uninstall.Notes, "\n")
	if !strings.Contains(notes, "preToolUse") || !strings.Contains(notes, "posição 0") || !strings.Contains(notes, "posição 1") {
		t.Fatalf("the moved handler is not reported: %q", notes)
	}
}

func cursorChangeHandlers(t *testing.T, plan Plan, mode string) {
	t.Helper()
	handlers := cursorHandlers(t, plan.Receipt.HooksPath)
	owned := handlers["preToolUse"][0]
	switch mode {
	case "changed":
		handlers["preToolUse"][0] = json.RawMessage(
			`{"command":"personal-check","matcher":"Write","timeout":10,"failClosed":true}`)
	case "duplicate":
		handlers["preToolUse"] = append(handlers["preToolUse"], owned)
	case "moved":
		foreign := json.RawMessage(`{"command":"first-check","matcher":"Write"}`)
		handlers["preToolUse"] = append([]json.RawMessage{foreign}, handlers["preToolUse"]...)
	}
	data, err := json.Marshal(map[string]any{"version": 1, "hooks": handlers})
	must(t, err)
	must(t, WriteAtomic(plan.Receipt.HooksPath, data))
}

func TestCursorUninstallRequiresItsReceipt(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		agent string
		bad   bool
	}{
		{"red another agent", "codex", true},
		{"near cursor", "cursor", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, plan := cursorInstalled(t, "user")
			receipt, err := ReadReceipt(plan.ReceiptPath)
			must(t, err)
			receipt.Agent = test.agent
			cursorWriteReceipt(t, plan.ReceiptPath, receipt)
			_, err = UninstallCursor(plan.ReceiptPath)
			if test.bad && !errors.Is(err, ErrPath) {
				t.Fatalf("receipt identity error = %v", err)
			}
			if !test.bad && err != nil {
				t.Fatal(err)
			}
		})
	}
}
