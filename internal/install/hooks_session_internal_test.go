package install

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestValidateHooksPinsTheSessionEntry(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../agents/claude/hooks/hooks.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateHooks(data); err != nil {
		t.Fatalf("the shipped hooks: %v", err)
	}
	handler := Handler{Type: "command", Command: "golaw", Args: []string{"hook", "claude"}, Timeout: 10, OnFailure: "block"}
	open := handler
	open.OnFailure = ""
	for name, change := range map[string]func(h *Hooks){
		"red: no session entry":          func(h *Hooks) { delete(h.Hooks, "SessionStart") },
		"red: a session matcher":         func(h *Hooks) { h.Hooks["SessionStart"] = []Group{{Matcher: "startup", Hooks: []Handler{handler}}} },
		"red: a session that fails open": func(h *Hooks) { h.Hooks["SessionStart"] = []Group{{Hooks: []Handler{open}}} },
	} {
		parsed, err := ParseHooks(data)
		if err != nil {
			t.Fatal(err)
		}
		change(&parsed)
		changed, err := json.Marshal(parsed)
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateHooks(changed); !errors.Is(err, ErrHooks) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
