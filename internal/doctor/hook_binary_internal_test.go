package doctor

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
)

// handlerNaming is a Codex handler whose commands name a binary the way
// the installer writes them on the two families of operating systems.
func handlerNaming(t *testing.T, posix, windows string) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"type": "command", "timeout": 10,
		"command":        install.HookCommand(posix, "darwin", "hook", "codex"),
		"commandWindows": install.HookCommand(windows, goosWindows, "hook", "codex"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCollectBinariesDecodesTheCommandTheHostRuns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	running, other := filepath.Join(dir, "golaw"), filepath.Join(dir, "other")
	writeFile(t, running, "binary")
	writeFile(t, other, "binary")
	handler := handlerNaming(t, running, other)
	slots := map[string][]slot{"PreToolUse": {{Raw: handler}}}
	receipt := install.Receipt{Entries: []install.Entry{
		{Event: "PreToolUse", Definition: handler},
		{Event: "PostToolUse", Definition: handler},
	}}
	if got := collectBinaries(receipt, slots, agentCodex, running, "darwin"); len(got) != 1 || got[0].Named != running || !got[0].Same || got[0].Error != "" {
		t.Fatalf("posix = %+v", got)
	}
	if got := collectBinaries(receipt, slots, agentCodex, running, goosWindows); len(got) != 1 || got[0].Named != other || got[0].Same {
		t.Fatalf("windows = %+v", got)
	}
	if got := collectBinaries(receipt, slots, agentCursor, running, goosWindows); len(got) != 1 || got[0].Named != running {
		t.Fatalf("cursor runs its one command = %+v", got)
	}
}

func TestCollectBinariesNamesWhatItCannotResolve(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	running := filepath.Join(dir, "golaw")
	writeFile(t, running, "binary")
	foreign := json.RawMessage(`{"type":"command","command":"golaw hook codex","timeout":10}`)
	gone := handlerNaming(t, filepath.Join(dir, "gone"), filepath.Join(dir, "gone"))
	slots := map[string][]slot{"PreToolUse": {{Raw: foreign}, {Raw: gone}}}
	receipt := install.Receipt{Entries: []install.Entry{{Event: "PreToolUse", Definition: foreign}, {Event: "PreToolUse", Definition: gone}}}
	got := collectBinaries(receipt, slots, agentCodex, running, "darwin")
	if len(got) != 2 || !strings.Contains(got[0].Error, "names no binary the installer wrote") || !strings.Contains(got[1].Error, "the hook names") {
		t.Fatalf("unresolved = %+v", got)
	}
	if _, err := sameFile(running, filepath.Join(dir, "absent")); err == nil || !strings.Contains(err.Error(), "the running binary") {
		t.Fatalf("an absent running binary = %v", err)
	}
}

func TestHookBinaryLinesJudgeEveryHandler(t *testing.T) {
	t.Parallel()
	facts := installedFacts(agentCursor, "user")
	if hookBinaryLines(facts) != nil {
		t.Fatal("a binary line without an installed handler")
	}
	for _, tc := range []struct {
		name     string
		binaries []binaryFact
		state    string
		code     string
		detail   string
	}{
		{"matches", []binaryFact{{Event: "preToolUse", Named: "bin/golaw", Same: true}}, stateOK, codeBinaryMatches, "bin/golaw"},
		{"differs", []binaryFact{{Event: "preToolUse", Named: "bin/golaw", Same: true}, {Event: "postToolUse", Named: "bin/old", Same: false}}, stateRed, codeBinaryDiffers, "postToolUse runs bin/old, not the binary running doctor"},
		{"unresolved", []binaryFact{{Event: "preToolUse", Error: "no binary"}, {Event: "postToolUse", Named: "bin/old"}}, stateRed, codeBinaryUnresolved, "preToolUse: no binary; run golaw agent install --agent cursor"},
	} {
		facts.Binaries = tc.binaries
		lines := hookBinaryLines(facts)
		if len(lines) != 1 || lines[0].ID != idBinary || lines[0].Key != "cursor/user" || lines[0].State != tc.state || lines[0].Code != tc.code || !strings.Contains(lines[0].Detail, tc.detail) {
			t.Errorf("%s: %+v", tc.name, lines)
		}
	}
}

func TestCodexReportNamesTheBinaryOfItsHook(t *testing.T) {
	t.Parallel()
	in := installCodex(t)
	running := filepath.Join(in.dir, "golaw")
	writeFile(t, running, "binary")
	handler := string(handlerNaming(t, running, running))
	writeFile(t, in.hooks, strings.ReplaceAll(codexHooks(""), codexHandler, handler))
	receipt, err := install.ReadReceipt(in.input.ReceiptPath)
	if err != nil {
		t.Fatal(err)
	}
	for index := range receipt.Entries {
		receipt.Entries[index].Definition = json.RawMessage(handler)
	}
	writeReceipt(t, in.input.ReceiptPath, receipt)
	in.input.RunningBinary = running
	line := lineWith(Codex(t.Context(), in.input, &enrollmentStore{}).Lines, idBinary)
	if line.State != stateOK || line.Code != codeBinaryMatches || line.Detail != running {
		t.Fatalf("binary = %+v", line)
	}
}
