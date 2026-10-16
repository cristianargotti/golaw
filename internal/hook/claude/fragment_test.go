package claude_test

import (
	"bytes"
	"encoding/json"
	"os"
	"slices"
	"testing"
)

type fragmentHandler struct {
	Type      string   `json:"type"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	Timeout   int      `json:"timeout"`
	OnFailure string   `json:"onFailure"`
}

type fragmentGroup struct {
	Matcher string            `json:"matcher"`
	Hooks   []fragmentHandler `json:"hooks"`
}

func TestPluginFragment(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../agents/claude/hooks/hooks.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Hooks map[string][]fragmentGroup `json:"hooks"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		t.Fatal(err)
	}
	expected := map[string][]string{
		"PreToolUse":         {"Write|Edit|NotebookEdit", "Read", "Bash|PowerShell"},
		"PostToolUse":        {"Write|Edit|NotebookEdit", "Bash|PowerShell"},
		"PostToolUseFailure": {"Bash|PowerShell"},
	}
	if len(document.Hooks) != len(expected) {
		t.Fatal("event count differs")
	}
	for event, matchers := range expected {
		checkGroups(t, document.Hooks[event], matchers)
	}
}

// checkGroups pins every handler to the exec form, the ten second timeout
// and onFailure block, so the host runs the binary without a shell and
// fails closed when it cannot answer.
func checkGroups(t *testing.T, groups []fragmentGroup, matchers []string) {
	t.Helper()
	if len(groups) != len(matchers) {
		t.Fatal("matcher count differs")
	}
	for i, group := range groups {
		if group.Matcher != matchers[i] || len(group.Hooks) != 1 {
			t.Fatalf("invalid group: %+v", group)
		}
		handler := group.Hooks[0]
		if handler.Type != "command" || handler.Command != "golaw" ||
			!slices.Equal(handler.Args, []string{"hook", "claude"}) ||
			handler.Timeout != 10 || handler.OnFailure != "block" {
			t.Fatalf("invalid handler: %+v", handler)
		}
	}
}
