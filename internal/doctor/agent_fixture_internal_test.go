package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
)

// The handlers an installation writes, as its receipt keeps them, and the
// matchers of their groups.
const (
	codexMatcher  = "^(Bash|apply_patch)$"
	codexHandler  = `{"type":"command","command":"'/opt/golaw' hook codex","timeout":10}`
	cursorHandler = `{"command":"'/opt/golaw' hook cursor","matcher":"Write|Delete|Shell","timeout":10,"failClosed":true}`
	rulesText     = "# rules\n"
	blockText     = blockBegin + "\nRead ~/.agents/skills/golaw/SKILL.md first.\n" + blockEnd
)

// installation is an agent installed under a temporary directory: the
// input doctor reads it with and the files it wrote.
type installation struct {
	input  Input
	dir    string
	hooks  string
	rules  string
	agents string
}

// writeFile writes a fixture file, its directories included.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// writeReceipt writes a receipt the way the installer does.
func writeReceipt(t *testing.T, path string, receipt install.Receipt) {
	t.Helper()
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, string(data)+"\n")
}

// codexHooks is a Codex hooks file holding golaw's two groups after the
// foreign groups given for PreToolUse.
func codexHooks(foreign string) string {
	group := `{"matcher":"` + codexMatcher + `","hooks":[` + codexHandler + `]}`
	pre := group
	if foreign != "" {
		pre = foreign + "," + group
	}
	return `{"hooks":{"PreToolUse":[` + pre + `],"PostToolUse":[` + group + `]}}`
}

// installCodex installs Codex at the user scope: the hooks file, the rules
// file, the AGENTS.md block and the receipt that lists them.
func installCodex(t *testing.T) installation {
	t.Helper()
	dir := t.TempDir()
	in := installation{
		dir: dir, hooks: filepath.Join(dir, "hooks.json"),
		rules: filepath.Join(dir, "rules", "golaw.rules"), agents: filepath.Join(dir, "AGENTS.md"),
	}
	writeFile(t, in.hooks, codexHooks(""))
	writeFile(t, in.rules, rulesText)
	writeFile(t, in.agents, "# Notes\n\n"+blockText+"\n")
	receipt := install.Receipt{
		Schema: 1, ProjectID: "repository", Scope: "user", Version: "0.4.0", Bundle: dir,
		Agent: agentCodex, HooksPath: in.hooks,
		Artifacts: []install.Artifact{{Path: in.rules, SHA256: install.Digest([]byte(rulesText))}},
		Entries: []install.Entry{
			{Event: "PreToolUse", Matcher: codexMatcher, Definition: json.RawMessage(codexHandler)},
			{Event: "PostToolUse", Matcher: codexMatcher, Definition: json.RawMessage(codexHandler)},
		},
		Blocks: []install.Block{{Path: in.agents, SHA256: install.Digest([]byte(blockText))}},
	}
	in.input = Input{Agent: agentCodex, Scope: "user", ReceiptPath: filepath.Join(dir, "receipt.json")}
	writeReceipt(t, in.input.ReceiptPath, receipt)
	return in
}

// installCursor installs Cursor at the user scope after a foreign handler
// of the same event.
func installCursor(t *testing.T) installation {
	t.Helper()
	dir := t.TempDir()
	in := installation{dir: dir, hooks: filepath.Join(dir, "hooks.json")}
	foreign := `{"command":"/usr/local/bin/audit","timeout":5}`
	writeFile(t, in.hooks, `{"version":1,"hooks":{"preToolUse":[`+foreign+`,`+cursorHandler+`]}}`)
	receipt := install.Receipt{
		Schema: 1, ProjectID: "repository", Scope: "user", Version: "0.4.0", Bundle: dir,
		Agent: agentCursor, HooksPath: in.hooks,
		Entries: []install.Entry{{Event: "preToolUse", Group: 1, Matcher: "Write|Delete|Shell", Definition: json.RawMessage(cursorHandler)}},
	}
	in.input = Input{Agent: agentCursor, Scope: "user", ReceiptPath: filepath.Join(dir, "receipt.json")}
	writeReceipt(t, in.input.ReceiptPath, receipt)
	return in
}
