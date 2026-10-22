package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReceiptKeepsTheFieldsOfEveryAgent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	receipt := Receipt{
		Schema: 1, ProjectID: "project", Scope: "user", Version: "0.4.0", Bundle: dir,
		Agent: "codex", HooksPath: filepath.Join(dir, "hooks.json"), HooksCreated: true,
		Artifacts: []Artifact{{Path: filepath.Join(dir, "rules", "golaw.rules"), SHA256: Digest([]byte("rules"))}},
		Entries: []Entry{{
			Event: "PreToolUse", Group: 1, Handler: 0, Matcher: "^(Bash|apply_patch)$",
			Definition: json.RawMessage(`{"type":"command","command":"\"/bin/golaw\" hook codex","timeout":10}`),
		}},
		Blocks: []Block{{Path: filepath.Join(dir, "AGENTS.md"), SHA256: Digest([]byte("block")), Created: true}},
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "receipt.json")
	if writeErr := WriteAtomic(path, append(data, '\n')); writeErr != nil {
		t.Fatal(writeErr)
	}
	got, err := ReadReceipt(path)
	if err != nil {
		t.Fatal(err)
	}
	// An indented receipt indents the raw definitions too, and a nil one
	// reads back as null, so a definition is compared as compact JSON, the
	// way doctor compares it.
	var compact bytes.Buffer
	if err := json.Compact(&compact, got.Entries[0].Definition); err != nil {
		t.Fatal(err)
	}
	got.Entries[0].Definition = compact.Bytes()
	if string(got.HookDefinition) != "null" {
		t.Fatalf("hook definition = %s", got.HookDefinition)
	}
	got.HookDefinition = nil
	if !reflect.DeepEqual(got, receipt) {
		t.Fatalf("got %+v, want %+v", got, receipt)
	}
}

func TestClaudeReceiptCarriesNoFieldOfTheOtherAgents(t *testing.T) {
	t.Parallel()
	data, err := json.Marshal(Receipt{Schema: 1, ProjectID: "project", Bundle: "/state"})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"agent"`, `"hooks_path"`, `"hooks_created"`, `"entries"`, `"blocks"`} {
		if strings.Contains(string(data), key) {
			t.Fatalf("a receipt without the fields of M4 writes %s: %s", key, data)
		}
	}
}

func TestPlannersOfAgentsNotBuiltRefuse(t *testing.T) {
	t.Parallel()
	receipt := filepath.Join(t.TempDir(), "receipt.json")
	for name, plan := range map[string]func() (Plan, error){
		"cursor uninstall": func() (Plan, error) { return UninstallCursor(receipt) },
	} {
		if _, err := plan(); !errors.Is(err, ErrNotBuilt) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
