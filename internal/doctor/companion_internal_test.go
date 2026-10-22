package doctor

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
)

// gateGroup is a Codex group whose handler runs the companion gate.
const gateGroup = `{"matcher":"Bash","hooks":[{"type":"command","command":"python3 /opt/gate/standards_gate.py","timeout":10}]}`

func TestRunsCompanionRecognizesTheScriptAsAWord(t *testing.T) {
	t.Parallel()
	for command, want := range map[string]bool{
		`{"command":"python3 /opt/gate/standards_gate.py"}`:                   true,
		`{"command":"'C:\\gate\\standards_gate.py' --hook"}`:                  true,
		`{"command":"golaw hook codex","commandWindows":"standards_gate.py"}`: true,
		`{"command":"python3 /opt/gate/standards_gate.pyc"}`:                  false,
		`{"command":"echo standards_gate.py"}`:                                false,
		`{"command":"python3 my_standards_gate.py"}`:                          false,
		`{"command":""}`: false,
	} {
		if got := runsCompanion(json.RawMessage(command)); got != want {
			t.Errorf("%s = %v", command, got)
		}
	}
}

func TestCollectCompanionsPlacesTheGateAgainstGolaw(t *testing.T) {
	t.Parallel()
	in := installCodex(t)
	writeFile(t, in.hooks, strings.Replace(codexHooks(gateGroup), `"PostToolUse":[`, `"PostToolUse":[`+gateGroup+`,`, 1))
	receipt, err := install.ReadReceipt(in.input.ReceiptPath)
	if err != nil {
		t.Fatal(err)
	}
	want := []companionFact{{Event: "PostToolUse", Path: in.hooks, Order: orderBefore}, {Event: "PreToolUse", Path: in.hooks, Order: orderBefore}}
	if got := collectCompanions(in.input, receipt); !reflect.DeepEqual(got, want) {
		t.Fatalf("before = %+v", got)
	}
	cursor := installCursor(t)
	writeFile(t, cursor.hooks, `{"version":1,"hooks":{"preToolUse":[`+cursorHandler+`,{"command":"standards_gate.py"}],"beforeReadFile":[{"command":"standards_gate.py"}]}}`)
	receipt, err = install.ReadReceipt(cursor.input.ReceiptPath)
	if err != nil {
		t.Fatal(err)
	}
	want = []companionFact{{Event: "beforeReadFile", Path: cursor.hooks, Order: orderAlone}, {Event: "preToolUse", Path: cursor.hooks, Order: orderAfter}}
	if got := collectCompanions(cursor.input, receipt); !reflect.DeepEqual(got, want) {
		t.Fatalf("after = %+v", got)
	}
}

func TestCollectCompanionsReadsTheHooksFileOfTheScopeWhenNothingIsInstalled(t *testing.T) {
	t.Parallel()
	input := homeAndRepository(t, agentCodex, "user")
	if got := collectCompanions(input, install.Receipt{}); got != nil {
		t.Fatalf("no hooks file = %+v", got)
	}
	writeFile(t, filepath.Join(input.ConfigDir, "hooks.json"), `{"hooks":{"PreToolUse":[`+gateGroup+`]}}`)
	if got := collectCompanions(input, install.Receipt{}); len(got) != 1 || got[0].Order != orderAlone {
		t.Fatalf("user scope = %+v", got)
	}
	input.Scope = scopeProject
	writeFile(t, filepath.Join(input.Root, ".codex", "hooks.json"), `{"hooks":{"PostToolUse":[`+gateGroup+`]}}`)
	if got := collectCompanions(input, install.Receipt{}); len(got) != 1 || got[0].Event != "PostToolUse" {
		t.Fatalf("project scope = %+v", got)
	}
	input.Agent = agentClaude
	if got := collectCompanions(input, install.Receipt{}); got != nil {
		t.Fatalf("the Claude report keeps doctor.python: %+v", got)
	}
}

func TestCompanionLinesNameTheOrder(t *testing.T) {
	t.Parallel()
	facts := installedFacts(agentCodex, "user")
	if lines := companionLines(facts); len(lines) != 1 || lines[0].State != stateOK || lines[0].Code != codeCompanionAbsent {
		t.Fatalf("absent = %+v", lines)
	}
	facts.Companions = []companionFact{
		{Event: "PreToolUse", Path: "hooks.json", Order: orderBefore},
		{Event: "PostToolUse", Path: "hooks.json", Order: orderAfter},
		{Event: "beforeReadFile", Path: "hooks.json", Order: orderAlone},
	}
	lines := companionLines(facts)
	for index, want := range []string{"before golaw in file order", "after golaw in file order", "where golaw has no handler"} {
		if lines[index].State != stateUnknown || lines[index].Code != codeCompanionFound || !strings.Contains(lines[index].Detail, want) {
			t.Errorf("line %d = %+v", index, lines[index])
		}
	}
	if companionLines(installedFacts(agentClaude, "user")) != nil {
		t.Fatal("the Claude report carries a companion line")
	}
}
