package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
)

// m4CursorHandler is a Cursor handler of golaw as the installer writes it.
func m4CursorHandler(command, matcher string) map[string]any {
	handler := map[string]any{"command": command, "timeout": hookTimeout, "failClosed": true}
	if matcher != "" {
		handler["matcher"] = matcher
	}
	return handler
}

// m4Installed writes the Codex and Cursor hooks files agent install
// --agent all leaves under home for the binary on goos: the foreign
// entries first, then golaw's.
func m4Installed(t *testing.T, home, binary, goos string) {
	t.Helper()
	codex := install.HookCommand(binary, goos, commandHook, agentCodex)
	handler := map[string]any{"type": keyCommand, "command": codex, "timeout": hookTimeout}
	if goos == goosWindows {
		handler["commandWindows"] = codex
	}
	group := map[string]any{"matcher": "^(Bash|apply_patch)$", "hooks": []any{handler}}
	cursor := install.HookCommand(binary, goos, commandHook, agentCursor)
	documents := map[string]any{
		m4HooksPath(home, agentCodex): map[string]any{"hooks": map[string]any{
			"PreToolUse": []any{json.RawMessage(foreignCodexGroup), group}, "PostToolUse": []any{group},
		}},
		m4HooksPath(home, agentCursor): map[string]any{"version": 1, "hooks": map[string]any{
			"preToolUse":           []any{m4CursorHandler(cursor, "Write|Delete|Shell")},
			"beforeReadFile":       []any{m4CursorHandler(cursor, "")},
			"beforeShellExecution": []any{json.RawMessage(foreignCursorHandle), m4CursorHandler(cursor, "")},
			"postToolUse":          []any{m4CursorHandler(cursor, "Write|Shell")},
			"postToolUseFailure":   []any{m4CursorHandler(cursor, "Write|Shell")},
		}},
	}
	for path, document := range documents {
		body, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		checkedWrite(t, path, string(body))
	}
}

// m4Rewrite replaces old with replacement in the hooks file of agent.
func m4Rewrite(t *testing.T, home, agent, old, replacement string) {
	t.Helper()
	path := m4HooksPath(home, agent)
	body, err := readFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.ReplaceAll(string(body), old, replacement)
	if changed == string(body) {
		t.Fatalf("%q is not in %s", old, path)
	}
	checkedWrite(t, path, changed)
}

func TestM4SelectsReadsMatchersLikeTheirHosts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		agent, matcher, tool string
		want                 bool
	}{
		{agentCodex, "^(Bash|apply_patch)$", "Bash", true},
		{agentCodex, "^(Bash|apply_patch)$", "apply_patch", true},
		{agentCodex, "^(Bash|apply_patch)$", "Write", false},
		{agentCodex, "Bash|apply_patch", "Bas", false},
		{agentCodex, "Bash|apply_patch", "apply_patch", true},
		{agentCodex, "", "mcp__server__tool", true},
		{agentCodex, "*", "spawn_agent", true},
		{agentCursor, "Write|Delete|Shell", "Shell", true},
		{agentCursor, "Write|Shell", "Delete", false},
	} {
		if got, err := m4Selects(tc.agent, tc.matcher, tc.tool); err != nil || got != tc.want {
			t.Errorf("%s %q %q = %v, %v", tc.agent, tc.matcher, tc.tool, got, err)
		}
	}
	if _, err := m4Selects(agentCursor, "(", "Shell"); err == nil {
		t.Fatal("an invalid matcher selected")
	}
}

func TestM4GolawFindsTheOneInstalledHandler(t *testing.T) {
	t.Parallel()
	for _, goos := range []string{"linux", goosWindows} {
		s := testSuite(t)
		s.m2OS = goos
		m4Installed(t, s.home, s.binary, goos)
		target := s.m4Shared(s.base)
		for _, probe := range [][3]string{
			{agentCodex, eventPre, toolBash},
			{agentCodex, eventPost, "apply_patch"},
			{agentCursor, "preToolUse", "Shell"},
			{agentCursor, "beforeShellExecution", ""},
			{agentCursor, "beforeReadFile", ""},
		} {
			entry, err := s.m4Golaw(target, probe[0], probe[1], probe[2])
			if err != nil || entry.Event != probe[1] || len(entry.Raw) == 0 {
				t.Fatalf("%s %v: %+v, %v", goos, probe, entry, err)
			}
		}
		if entry, err := s.m4Golaw(target, agentCodex, eventPre, toolBash); err != nil || entry.Group != 1 || entry.Index != 0 {
			t.Fatalf("%s: golaw's Codex group after the foreign one = %+v, %v", goos, entry, err)
		}
		if _, err := s.m4Golaw(target, agentCodex, eventPre, "Write"); err == nil || !strings.Contains(err.Error(), "0 installed handlers") {
			t.Fatalf("%s: an unselected tool = %v", goos, err)
		}
	}
}

func TestM4GolawRefusesWhatTheInstallerNeverWrites(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, agent, old, replacement, event, tool, want string
	}{
		{"red a Codex timeout of 9", agentCodex, `"timeout":10`, `"timeout":9`, eventPre, toolBash, "timeout 9"},
		{"red a Codex fail closed field", agentCodex, `"timeout":10`, `"timeout":10,"failClosed":true`, eventPre, toolBash, "without a fail closed field"},
		{"red an async Codex handler", agentCodex, `"timeout":10`, `"timeout":10,"async":true`, eventPre, toolBash, "blocking command"},
		{"red a Cursor handler failing open", agentCursor, `"failClosed":true`, `"failClosed":false`, "beforeReadFile", "", "does not set failClosed"},
		{"red the binary of another path", agentCursor, executable(binaryName) + "' hook cursor", "missing' hook cursor", "beforeReadFile", "", "want one"},
		{"red an undecodable hooks file", agentCodex, `{"hooks"`, `{"hooks":1,"x"`, eventPre, toolBash, "decode"},
	} {
		s := testSuite(t)
		m4Installed(t, s.home, s.binary, "linux")
		s.m2OS = "linux"
		m4Rewrite(t, s.home, tc.agent, tc.old, tc.replacement)
		if _, err := s.m4Golaw(s.m4Shared(s.base), tc.agent, tc.event, tc.tool); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
	s := testSuite(t)
	if _, err := s.m4Golaw(s.m4Shared(s.base), agentCodex, eventPre, toolBash); err == nil {
		t.Fatal("an absent hooks file passed")
	}
}

// m4RequireShells holds the commands one replay ran to the shells of the
// system, the installed command line, the working directory of the agent,
// the payload on stdin and the environment of the target.
func m4RequireShells(t *testing.T, s *suite, agent string, shells []string, seen []command, replies []m4Reply, wantDir string) {
	t.Helper()
	if len(replies) != len(shells) || len(seen) != len(shells) {
		t.Fatalf("%s: %d replies of %d commands, want %d", agent, len(replies), len(seen), len(shells))
	}
	line := install.HookCommand(s.binary, s.m4System(), commandHook, agent)
	for i, c := range seen {
		words := strings.Fields(shells[i])
		if c.program != words[0] || !slices.Equal(c.args, append(words[1:], line)) || c.dir != wantDir ||
			c.input != `{"x":1}` || !slices.Equal(c.env, s.env) || replies[i].shell != c.program {
			t.Fatalf("%s: %+v", agent, c)
		}
	}
}

func TestM4InvokeRunsTheCommandUnderEveryShell(t *testing.T) {
	t.Parallel()
	for goos, shells := range map[string][]string{
		"linux":     {"sh -c"},
		goosWindows: {"cmd.exe /c", "powershell.exe -NoProfile -Command"},
	} {
		s := testSuite(t)
		s.m2OS = goos
		m4Installed(t, s.home, s.binary, goos)
		var seen []command
		s.execute = func(_ context.Context, c command) result {
			seen = append(seen, c)
			return result{}
		}
		repo := filepath.Join(s.base, "repo")
		seen = nil
		replies, err := s.m4Invoke(t.Context(), s.m4Shared(repo), agentCodex, eventPre, toolBash, []byte(`{"x":1}`))
		if err != nil {
			t.Fatal(err)
		}
		m4RequireShells(t, s, agentCodex, shells, seen, replies, repo)
		seen = nil
		replies, err = s.m4Invoke(t.Context(), s.m4Shared(repo), agentCursor, "preToolUse", "Shell", []byte(`{"x":1}`))
		if err != nil {
			t.Fatal(err)
		}
		m4RequireShells(t, s, agentCursor, shells, seen, replies, filepath.Join(s.home, ".cursor"))
	}
	s := testSuite(t)
	m4Installed(t, s.home, s.binary, "linux")
	s.m2OS = "linux"
	s.lookup = func(string) (string, error) { return "", ErrAcceptance }
	if _, err := s.m4Invoke(t.Context(), s.m4Shared(s.base), agentCodex, eventPre, toolBash, nil); err == nil || !strings.Contains(err.Error(), "locate sh") {
		t.Fatalf("an absent shell = %v", err)
	}
}
