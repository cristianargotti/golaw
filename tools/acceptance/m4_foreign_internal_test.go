package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
)

// m4GolawGroup is the group golaw's installer appends for an event: the
// handler naming the binary the way the installer writes it.
func m4GolawGroup(t *testing.T, binary string) string {
	t.Helper()
	command, err := json.Marshal(install.HookCommand(binary, runtime.GOOS, "hook", agentCodex))
	if err != nil {
		t.Fatal(err)
	}
	return `{"matcher":"^(Bash|apply_patch)$","hooks":[{"type":"command","command":` + string(command) + `,"timeout":10}]}`
}

func TestInstallArgsInstallEveryAgent(t *testing.T) {
	t.Parallel()
	if got := installArgs(); !slices.Equal(got, []string{"agent", "install", "--agent", "all"}) {
		t.Fatalf("install args = %q", got)
	}
}

func TestForeignEntriesSurviveInPlace(t *testing.T) {
	t.Parallel()
	golaw := m4GolawGroup(t, "/opt/golaw")
	cursorGolaw := `{"command":"'/opt/golaw' hook cursor","timeout":10,"failClosed":true}`
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"near golaw appended after every foreign entry", map[string]string{
			".codex/hooks.json":  `{"hooks":{"PreToolUse":[` + foreignCodexGroup + `,` + golaw + `]}}`,
			".codex/AGENTS.md":   foreignAgents + "\n<!-- golaw:begin -->\nx\n<!-- golaw:end -->\n",
			".cursor/hooks.json": `{"version":1,"hooks":{"beforeShellExecution":[` + foreignCursorHandle + `,` + cursorGolaw + `]}}`,
		}, ""},
		{"red the foreign group moved after golaw's", map[string]string{
			".codex/hooks.json": `{"hooks":{"PreToolUse":[` + golaw + `,` + foreignCodexGroup + `]}}`,
		}, "foreign PreToolUse entry"},
		{"red the foreign group rewritten", map[string]string{
			".codex/hooks.json": `{"hooks":{"PreToolUse":[` + strings.Replace(foreignCodexGroup, `"timeout":5`, `"timeout":6`, 1) + `]}}`,
		}, "foreign PreToolUse entry"},
		{"red the block put before the person's text", map[string]string{
			".codex/AGENTS.md": "<!-- golaw:begin -->\nx\n<!-- golaw:end -->\n" + foreignAgents,
		}, "no longer heads"},
		{"red the foreign Cursor handler dropped", map[string]string{
			".cursor/hooks.json": `{"version":1,"hooks":{"beforeShellExecution":[` + cursorGolaw + `]}}`,
		}, "foreign beforeShellExecution entry"},
		{"red an undecodable hooks file", map[string]string{".cursor/hooks.json": "{"}, "decode"},
	} {
		home := t.TempDir()
		if err := seedForeign(home); err != nil {
			t.Fatal(err)
		}
		for rel, body := range tc.files {
			checkedWrite(t, filepath.Join(home, filepath.FromSlash(rel)), body)
		}
		err := preservedForeign(home)
		if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
	if err := preservedForeign(t.TempDir()); err == nil {
		t.Fatal("absent agent files passed")
	}
}

func TestInstallSeedsAndKeepsForeignEntries(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	if err := preservedForeign(s.home); err != nil {
		t.Fatalf("the prepared home lost a foreign entry: %v", err)
	}
	base := s.execute
	s.execute = func(ctx context.Context, c command) result {
		got := base(ctx, c)
		if len(c.args) > 0 && c.args[0] == "agent" {
			checkedWrite(t, m4HooksPath(environmentValue(c.env, "HOME"), agentCodex), `{"hooks":{}}`)
		}
		return got
	}
	if err := s.install(t.Context()); err == nil || !strings.Contains(err.Error(), "foreign PreToolUse entry") {
		t.Fatalf("an installer that drops a foreign group = %v", err)
	}
}

func TestFindReceiptReadsTheClaudeReceiptAmongAgents(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	claude := filepath.Join(home, "state", "golaw", agentClaude, "user", receiptFile)
	checkedWrite(t, claude, `{"schema":1}`)
	checkedWrite(t, filepath.Join(home, "state", "golaw", agentCodex, "user", receiptFile), `{"schema":1,"agent":"codex"}`)
	checkedWrite(t, filepath.Join(home, "state", "golaw", agentCursor, "user", receiptFile), `{"schema":1,"agent":"cursor"}`)
	if got, err := findReceipt(home); err != nil || got != claude {
		t.Fatalf("receipt = %q, %v", got, err)
	}
	checkedWrite(t, filepath.Join(home, "state", "other", receiptFile), `{"schema":1,"agent":"claude"}`)
	if _, err := findReceipt(home); err == nil || !strings.Contains(err.Error(), "found 2 receipts of claude") {
		t.Fatalf("two Claude receipts = %v", err)
	}
	if err := os.Remove(claude); err != nil {
		t.Fatal(err)
	}
	checkedWrite(t, filepath.Join(home, "state", "other", receiptFile), "{")
	if _, err := findReceipt(home); err == nil || !strings.Contains(err.Error(), "decode the receipt") {
		t.Fatalf("an undecodable receipt = %v", err)
	}
}
