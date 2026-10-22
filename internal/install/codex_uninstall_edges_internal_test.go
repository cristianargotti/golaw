package install

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestCodexUninstallKeepsWhatOthersAddedToItsGroup removes golaw's handler
// alone from a group another tool also wrote into, and keeps a hooks file
// the install created while something of someone else is left in it.
func TestCodexUninstallKeepsWhatOthersAddedToItsGroup(t *testing.T) {
	t.Parallel()
	options := codexOptions(t, "user")
	installed := codexInstall(t, options)
	document := codexDecode(t, cinRead(t, installed.Receipt.HooksPath))
	group := codexFirstGroup(t, document, "PostToolUse")
	handlers, ok := group["hooks"].([]any)
	if !ok {
		t.Fatalf("handlers = %T", group["hooks"])
	}
	foreign := map[string]any{"type": "command", "command": "other", "timeout": float64(3)}
	group["hooks"] = append(handlers, foreign)
	pre := codexFirstGroup(t, document, "PreToolUse")
	pre["note"] = "keep"
	cinWrite(t, installed.Receipt.HooksPath, codexJSON(t, document))
	plan, err := UninstallCodex(installed.ReceiptPath)
	must(t, err)
	must(t, Apply(t.Context(), plan, io.Discard, nil))
	after := codexDecode(t, cinRead(t, installed.Receipt.HooksPath))
	kept := codexFirstGroup(t, after, "PostToolUse")
	if !reflect.DeepEqual(kept["hooks"], []any{foreign}) || kept["matcher"] != "^(Bash|apply_patch)$" {
		t.Fatalf("post group = %#v", kept)
	}
	note := codexFirstGroup(t, after, "PreToolUse")
	if left, isList := note["hooks"].([]any); !isList || len(left) != 0 || note["note"] != "keep" {
		t.Fatalf("pre group = %#v", note)
	}
	if _, statErr := os.Stat(installed.ReceiptPath); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("receipt remains: %v", statErr)
	}
}

// TestCodexUninstallAfterTheHandlerWasRemoved removes the handler the
// person left and needs nothing for the one the person already removed,
// while a handler the person edited is refused.
func TestCodexUninstallAfterTheHandlerWasRemoved(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		edit func(*testing.T, map[string]any)
		want error
	}{
		{"removed", func(t *testing.T, document map[string]any) {
			t.Helper()
			events, ok := document["hooks"].(map[string]any)
			if !ok {
				t.Fatalf("hooks = %T", document["hooks"])
			}
			delete(events, "PreToolUse")
		}, nil},
		{"edited command", func(t *testing.T, document map[string]any) {
			t.Helper()
			handlers, ok := codexFirstGroup(t, document, "PreToolUse")["hooks"].([]any)
			if !ok || len(handlers) == 0 {
				t.Fatal("no handler to edit")
			}
			handler, ok := handlers[0].(map[string]any)
			if !ok {
				t.Fatalf("handler = %T", handlers[0])
			}
			handler["command"] = fmt.Sprint(handler["command"]) + " --verbose"
		}, ErrChanged},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			codexUninstallEdited(t, row.edit, row.want)
		})
	}
}

// codexUninstallEdited installs, lets edit change the hooks file and
// plans the removal: a hooks file the install created is removed once
// nothing is left in it, here the PostToolUse handler alone.
func codexUninstallEdited(t *testing.T, edit func(*testing.T, map[string]any), want error) {
	t.Helper()
	options := codexOptions(t, "user")
	installed := codexInstall(t, options)
	document := codexDecode(t, cinRead(t, installed.Receipt.HooksPath))
	edit(t, document)
	cinWrite(t, installed.Receipt.HooksPath, codexJSON(t, document))
	plan, err := UninstallCodex(installed.ReceiptPath)
	if !errors.Is(err, want) {
		t.Fatalf("uninstall = %v, want %v", err, want)
	}
	if want == nil && (len(plan.Changes) == 0 || plan.Changes[0].Path != installed.Receipt.HooksPath || !plan.Changes[0].Remove) {
		t.Fatalf("changes = %+v", plan.Changes)
	}
}

// TestCodexUninstallOfWhatIsAlreadyGone removes what remains when the
// person already deleted the hooks file, the rules and the instruction
// file: nothing is left to refuse.
func TestCodexUninstallOfWhatIsAlreadyGone(t *testing.T) {
	t.Parallel()
	options := codexOptions(t, "project")
	installed := codexInstall(t, options)
	for _, path := range []string{installed.Receipt.HooksPath, codexRulesPath(installed.Receipt), installed.Receipt.Blocks[0].Path} {
		must(t, os.Remove(path))
	}
	plan, err := UninstallCodex(installed.ReceiptPath)
	must(t, err)
	if len(plan.Changes) != 2 || plan.Changes[0].Path != SkillPath(options) || plan.Changes[1].Path != installed.ReceiptPath {
		t.Fatalf("changes = %+v", plan.Changes)
	}
}

// TestCodexUninstallRefusesEvidenceItDidNotWrite refuses a receipt kept
// where no Codex installation keeps one, or naming files no Codex
// installation writes, and a hooks file Codex could not read, before
// anything is removed.
func TestCodexUninstallRefusesEvidenceItDidNotWrite(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		edit func(*Receipt) string
	}{
		{"another agent", func(r *Receipt) string { r.Agent = "cursor"; return "" }},
		{"unknown scope", func(r *Receipt) string { r.Scope = "elsewhere"; return "" }},
		{"moved receipt", func(r *Receipt) string { r.Bundle += "-moved"; return "" }},
		{"another identity", func(r *Receipt) string { r.ProjectID = "other"; return "" }},
		{"relative hooks", func(r *Receipt) string { r.HooksPath = "hooks.json"; return "" }},
		{"unknown artifact", func(r *Receipt) string {
			r.Artifacts = append(r.Artifacts, Artifact{Path: filepath.Join(r.Bundle, "notes.md")})
			return ""
		}},
		{"unknown block", func(r *Receipt) string { r.Blocks[0].Path += ".old"; return "" }},
		{"unreadable hooks", func(r *Receipt) string { return r.HooksPath }},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			options := codexOptions(t, "project")
			installed := codexInstall(t, options)
			receipt := installed.Receipt
			if broken := row.edit(&receipt); broken != "" {
				cinWrite(t, broken, []byte(`{"hooks":`))
			}
			cinWrite(t, installed.ReceiptPath, codexJSON(t, receipt))
			if _, err := UninstallCodex(installed.ReceiptPath); !errors.Is(err, ErrSettings) {
				t.Fatalf("uninstall = %v", err)
			}
		})
	}
}
