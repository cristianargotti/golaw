package doctor

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
)

func TestCollectAgentReadsAnIntactInstallation(t *testing.T) {
	t.Parallel()
	in := installCodex(t)
	in.input.Dispatch = Dispatch{State: DispatchObserved, OS: "linux"}
	facts := collectAgent(in.input)
	if facts.Agent != agentCodex || facts.Scope != "user" || facts.Receipt.State != receiptRead || len(facts.Receipt.Drift) != 0 {
		t.Fatalf("facts = %+v", facts)
	}
	want := []eventFact{{Event: "PostToolUse", Present: true}, {Event: "PreToolUse", Present: true}}
	if !reflect.DeepEqual(facts.Events, want) || facts.Dispatch.State != DispatchObserved {
		t.Fatalf("events = %+v, dispatch = %+v", facts.Events, facts.Dispatch)
	}
	cursor := collectAgent(installCursor(t).input)
	if len(cursor.Receipt.Drift) != 0 || !reflect.DeepEqual(cursor.Events, []eventFact{{Event: "preToolUse", Present: true, FailClosed: true}}) {
		t.Fatalf("a handler after a foreign one = %+v", cursor)
	}
}

func TestCollectAgentNamesEveryDriftedPart(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		edit  func(t *testing.T, in installation)
		drift string
	}{
		{"artifact", func(t *testing.T, in installation) { t.Helper(); writeFile(t, in.rules, "# edited\n") }, "the artifact " + "RULES"},
		{"artifact gone", func(t *testing.T, in installation) {
			t.Helper()
			if err := os.Remove(in.rules); err != nil {
				t.Fatal(err)
			}
		}, "the artifact " + "RULES"},
		{"entry", func(t *testing.T, in installation) {
			t.Helper()
			writeFile(t, in.hooks, strings.Replace(codexHooks(""), `"timeout":10`, `"timeout":11`, 1))
		}, "the PreToolUse handler in " + "HOOKS"},
		{"hooks file gone", func(t *testing.T, in installation) {
			t.Helper()
			if err := os.Remove(in.hooks); err != nil {
				t.Fatal(err)
			}
		}, "the PostToolUse handler in " + "HOOKS"},
		{"block", func(t *testing.T, in installation) {
			t.Helper()
			writeFile(t, in.agents, strings.Replace(blockText, "first", "later", 1))
		}, "the marked block in " + "AGENTS"},
		{"block markers gone", func(t *testing.T, in installation) { t.Helper(); writeFile(t, in.agents, "# Notes\n") }, "the marked block in " + "AGENTS"},
		{"matcher only", func(t *testing.T, in installation) {
			t.Helper()
			writeFile(t, in.hooks, strings.Replace(codexHooks(""), `"matcher":"`+codexMatcher+`"`, `"matcher":"^NeverMatches$"`, 1))
		}, "the PreToolUse handler in " + "HOOKS"},
	} {
		in := installCodex(t)
		tc.edit(t, in)
		want := strings.NewReplacer("RULES", in.rules, "HOOKS", in.hooks, "AGENTS", in.agents).Replace(tc.drift)
		facts := collectAgent(in.input)
		if facts.Receipt.State != receiptRead || !slices.Contains(facts.Receipt.Drift, want) {
			t.Errorf("%s: drift = %q, want %q", tc.name, facts.Receipt.Drift, want)
		}
	}
}

func TestCollectAgentKeepsAReformattedFileAndRefusesAForeignReceipt(t *testing.T) {
	t.Parallel()
	in := installCodex(t)
	var indented bytes.Buffer
	if err := json.Indent(&indented, []byte(codexHooks("")), "", "    "); err != nil {
		t.Fatal(err)
	}
	writeFile(t, in.hooks, indented.String())
	if facts := collectAgent(in.input); len(facts.Receipt.Drift) != 0 {
		t.Fatalf("an indented hooks file drifted: %q", facts.Receipt.Drift)
	}
	writeFile(t, in.hooks, codexHooks(`{"matcher":"Bash","hooks":[{"type":"command","command":"other-kit hook"}]}`))
	if facts := collectAgent(in.input); len(facts.Receipt.Drift) != 0 {
		t.Fatalf("a foreign group before golaw's drifted: %q", facts.Receipt.Drift)
	}
	in.input.Agent = agentCursor
	if facts := collectAgent(in.input); !slices.Contains(facts.Receipt.Drift, `the receipt belongs to the agent "codex"`) {
		t.Fatalf("a receipt of another agent = %q", facts.Receipt.Drift)
	}
}

// TestCollectAgentKeepsReorderedHandlerKeys holds doctor to the comparison
// the installers make: a handler whose keys another tool reordered is the
// same handler, for Codex and for Cursor, and never installed-drift.
func TestCollectAgentKeepsReorderedHandlerKeys(t *testing.T) {
	t.Parallel()
	codex := installCodex(t)
	reordered := `{"timeout":10,"command":"'/opt/golaw' hook codex","type":"command"}`
	writeFile(t, codex.hooks, strings.ReplaceAll(codexHooks(""), codexHandler, reordered))
	if facts := collectAgent(codex.input); len(facts.Receipt.Drift) != 0 || len(facts.Events) != 2 || !facts.Events[0].Present || !facts.Events[1].Present {
		t.Fatalf("a Codex handler with reordered keys drifted: %+v", facts)
	}
	cursor := installCursor(t)
	reordered = `{"failClosed":true,"timeout":10,"matcher":"Write|Delete|Shell","command":"'/opt/golaw' hook cursor"}`
	writeFile(t, cursor.hooks, `{"version":1,"hooks":{"preToolUse":[{"command":"/usr/local/bin/audit","timeout":5},`+reordered+`]}}`)
	if facts := collectAgent(cursor.input); len(facts.Receipt.Drift) != 0 || !reflect.DeepEqual(facts.Events, []eventFact{{Event: "preToolUse", Present: true, FailClosed: true}}) {
		t.Fatalf("a Cursor handler with reordered keys drifted: %+v", facts)
	}
}

func TestCollectAgentTellsAnAbsentReceiptFromAnUnreadableOne(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	absent := collectAgent(Input{Agent: agentCodex, Scope: "user", ReceiptPath: filepath.Join(dir, "receipt.json")})
	if absent.Receipt.State != receiptAbsent || absent.Receipt.Error != "" || absent.Events != nil {
		t.Fatalf("absent = %+v", absent)
	}
	broken := filepath.Join(dir, "broken.json")
	writeFile(t, broken, "{")
	invalid := collectAgent(Input{Agent: agentCodex, Scope: "user", ReceiptPath: broken})
	if invalid.Receipt.State != receiptInvalid || invalid.Receipt.Error == "" {
		t.Fatalf("invalid = %+v", invalid)
	}
}

func TestEventFactsJoinTheHandlersOfOneEvent(t *testing.T) {
	t.Parallel()
	slots, err := parseSlots([]byte(`{"version":1,"hooks":{"postToolUse":[`+cursorHandler+`]}}`), true)
	if err != nil {
		t.Fatal(err)
	}
	receipt := install.Receipt{Entries: []install.Entry{
		{Event: "postToolUse", Definition: json.RawMessage(cursorHandler)},
		{Event: "postToolUse", Definition: json.RawMessage(`{"command":"gone"}`)},
	}}
	if got := eventFacts(receipt, slots); !reflect.DeepEqual(got, []eventFact{{Event: "postToolUse"}}) {
		t.Fatalf("events = %+v", got)
	}
	if _, err := parseSlots([]byte("["), true); err == nil {
		t.Fatal("a broken flat file was read")
	}
	if _, err := parseSlots([]byte("["), false); err == nil {
		t.Fatal("a broken grouped file was read")
	}
	if locate(slots["postToolUse"], json.RawMessage("{")) != -1 {
		t.Fatal("a broken definition was located")
	}
	if fields := fieldsOf(json.RawMessage("[")); fields != (handlerFields{}) {
		t.Fatalf("fields of a list = %+v", fields)
	}
}

func TestClaudeFactsReadTheFrameFromTheM1Evidence(t *testing.T) {
	t.Parallel()
	snapshot := baseline(t)
	path := filepath.Join(t.TempDir(), "receipt.json")
	writeReceipt(t, path, install.Receipt{Schema: 1, ProjectID: "repository", Bundle: "bundle"})
	facts := claudeFacts(Input{Scope: "user", ReceiptPath: path}, snapshot)
	if facts.Agent != agentClaude || facts.Receipt.State != receiptRead || len(facts.Receipt.Drift) != 0 || len(facts.Events) != 3 {
		t.Fatalf("facts = %+v", facts)
	}
	for _, event := range facts.Events {
		if !event.Present || !event.FailClosed {
			t.Errorf("event = %+v", event)
		}
	}
	snapshot.Files["asset"] = "changed"
	if drift := claudeFacts(Input{ReceiptPath: path}, snapshot).Receipt.Drift; !slices.Contains(drift, "doctor.artifacts asset artifact-drift") {
		t.Fatalf("drift = %q", drift)
	}
	softer := strings.Replace(snapshot.Hooks, `"onFailure":"block"`, `"onFailure":""`, 1)
	if events := claudeEvents(softer); len(events) == 0 || events[0].FailClosed && events[1].FailClosed && events[2].FailClosed {
		t.Fatalf("a handler that does not block = %+v", events)
	}
	if claudeEvents("{") != nil {
		t.Fatal("a broken plugin hooks file has events")
	}
}
