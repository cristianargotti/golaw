package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// The agents whose hooks files the installer merges, and the format of
// every decoding failure of a file the harness reads.
const (
	agentCodex   = "codex"
	agentCursor  = "cursor"
	decodeFormat = "decode %s: %w"
)

// The Cursor events golaw handles (D15).
const (
	cursorPreTool     = "preToolUse"
	cursorReadFile    = "beforeReadFile"
	cursorShellExec   = "beforeShellExecution"
	cursorPostTool    = "postToolUse"
	cursorPostFailure = "postToolUseFailure"
)

// The entries another tool owns in the files the installer merges: a group
// of another kit in the Codex hooks file, the person's own text in the
// Codex AGENTS.md and a handler of another gate in the Cursor hooks file.
// Each must survive two installations in place and in order (section 10.3
// item 2).
const (
	foreignCodexGroup   = `{"matcher":"^Bash$","hooks":[{"type":"command","command":"foreign-kit hook","timeout":5}]}`
	foreignAgents       = "# Personal notes\n\nKeep answers short.\n"
	foreignCursorHandle = `{"command":"foreign-gate","timeout":5}`
	agentsFile          = "AGENTS.md"
)

// m4HooksPath is the user hooks file of an agent under a home: the Codex
// one under CODEX_HOME, which the isolated environment puts at .codex, and
// the Cursor one under .cursor.
func m4HooksPath(home, agent string) string {
	return filepath.Join(home, "."+agent, hooksFileName)
}

// seedForeign writes the foreign entries before the first installation.
func seedForeign(home string) error {
	files := map[string]string{
		m4HooksPath(home, agentCodex):             `{"hooks":{"PreToolUse":[` + foreignCodexGroup + `]}}`,
		filepath.Join(home, codexDir, agentsFile): foreignAgents,
		m4HooksPath(home, agentCursor):            `{"version":1,"hooks":{"` + cursorShellExec + `":[` + foreignCursorHandle + `]}}`,
	}
	for path, body := range files {
		if err := writeFile(path, []byte(body), fileMode); err != nil {
			return err
		}
	}
	return nil
}

// preservedForeign holds every foreign entry to its place: still the first
// of its list, byte for byte as compact JSON, and the person's text still
// the head of AGENTS.md, so an installation that reorders, rewrites or
// drops an entry it does not own is a failure.
func preservedForeign(home string) error {
	if err := firstEntry(m4HooksPath(home, agentCodex), "PreToolUse", foreignCodexGroup); err != nil {
		return err
	}
	if err := firstEntry(m4HooksPath(home, agentCursor), cursorShellExec, foreignCursorHandle); err != nil {
		return err
	}
	body, err := readFile(filepath.Join(home, codexDir, agentsFile))
	if err != nil {
		return err
	}
	if !strings.HasPrefix(string(body), foreignAgents) {
		return fmt.Errorf("the text of the person no longer heads the Codex %s: %w", agentsFile, ErrAcceptance)
	}
	return nil
}

// firstEntry requires the first entry of an event's list in a hooks file to
// be want.
func firstEntry(path, event, want string) error {
	body, err := readFile(path)
	if err != nil {
		return err
	}
	var document struct {
		Hooks map[string][]json.RawMessage `json:"hooks"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return fmt.Errorf(decodeFormat, path, err)
	}
	entries := document.Hooks[event]
	if len(entries) == 0 || !sameJSON(entries[0], []byte(want)) {
		return fmt.Errorf("the foreign %s entry of %s moved or changed: %w", event, path, ErrAcceptance)
	}
	return nil
}

// sameJSON compares two documents as compact JSON.
func sameJSON(a, b []byte) bool {
	var left, right bytes.Buffer
	if json.Compact(&left, a) != nil || json.Compact(&right, b) != nil {
		return false
	}
	return bytes.Equal(left.Bytes(), right.Bytes())
}
