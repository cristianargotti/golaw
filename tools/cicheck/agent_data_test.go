package cicheck_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

type hookHandler struct {
	Type      string   `json:"type"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	Timeout   int      `json:"timeout"`
	OnFailure string   `json:"onFailure"`
}

type hookGroup struct {
	Matcher string        `json:"matcher"`
	Hooks   []hookHandler `json:"hooks"`
}

func agentID(t *testing.T) string {
	t.Helper()
	paths, err := filepath.Glob("../../agents/*/hooks/hooks.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected one agent hook asset, found %d", len(paths))
	}
	return filepath.Base(filepath.Dir(filepath.Dir(paths[0])))
}

func agentHandlerJSON(t *testing.T) string {
	t.Helper()
	path := filepath.ToSlash(filepath.Join("agents", agentID(t), "hooks", "hooks.json"))
	var document struct {
		Hooks map[string][]hookGroup `json:"hooks"`
	}
	if err := json.Unmarshal(kitBytes(t, path), &document); err != nil {
		t.Fatal(err)
	}
	groups := document.Hooks["PreToolUse"]
	if len(groups) == 0 || len(groups[0].Hooks) == 0 {
		t.Fatal("pre event handler is absent")
	}
	data, err := json.MarshalIndent(groups[0].Hooks[0], "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
