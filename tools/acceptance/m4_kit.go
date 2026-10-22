package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/install"
)

// The stub second kit of A13: its source, the text it refuses and the
// event each agent's file holds its entry under.
const (
	m4KitSource = "tools/acceptance/testdata/m4/kit.go.txt"
	m4KitName   = "m4-kit"
	m4KitMarker = "second-kit-deny"
)

// buildProgram builds a program of the harness from its source under a
// directory of the base named after it.
func (s *suite) buildProgram(ctx context.Context, source, name string) (string, error) {
	body, err := readFile(source)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(s.base, name)
	main := filepath.Join(dir, "main.go")
	if err := writeFile(main, body, fileMode); err != nil {
		return "", err
	}
	binary := filepath.Join(dir, executable(name))
	got := s.execute(ctx, command{
		program: s.goTool, args: []string{goBuild, "-o", binary, main},
		dir: s.root, env: s.env,
	})
	if err := success(got); err != nil {
		return "", fmt.Errorf("build %s: %w", name, err)
	}
	return binary, nil
}

// m4KitEvent is the event of an agent's hooks file the second kit and
// golaw both hold a shell entry under.
func m4KitEvent(agent string) string {
	if agent == agentCursor {
		return cursorShellExec
	}
	return eventPre
}

// m4KitEntry is the entry the second kit's installer writes: a Codex group
// of one blocking handler, or a Cursor handler failing closed, its command
// the kit's binary through the same quoting golaw's installer uses.
func m4KitEntry(agent, kit, goos string) map[string]any {
	line := install.HookCommand(kit, goos, agent)
	if agent == agentCursor {
		return map[string]any{"command": line, "timeout": hookTimeout, "failClosed": true}
	}
	handler := map[string]any{"type": keyCommand, "command": line, "timeout": hookTimeout}
	if goos == goosWindows {
		handler["commandWindows"] = line
	}
	return map[string]any{"matcher": "^(Bash|apply_patch)$", "hooks": []any{handler}}
}

// m4AddKit writes the second kit's entry into an agent's hooks file,
// ahead of every entry or after them, keeping every other field.
func m4AddKit(home, agent, kit, goos string, first bool) error {
	path := m4HooksPath(home, agent)
	document := map[string]any{}
	body, err := readFile(path)
	switch {
	case err == nil:
		if decodeErr := json.Unmarshal(body, &document); decodeErr != nil {
			return fmt.Errorf(decodeFormat, path, decodeErr)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	if agent == agentCursor && document["version"] == nil {
		document["version"] = 1
	}
	hooks, ok := document["hooks"].(map[string]any)
	if !ok {
		hooks = map[string]any{}
	}
	list, found := hooks[m4KitEvent(agent)].([]any)
	if !found {
		list = nil
	}
	entry := m4KitEntry(agent, kit, goos)
	if first {
		list = slices.Insert(list, 0, any(entry))
	} else {
		list = append(list, entry)
	}
	hooks[m4KitEvent(agent)], document["hooks"] = list, hooks
	out, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return writeFile(path, out, fileMode)
}

// m4HostBlocks is what the host does with one reply, from the host's side
// of the protocol (survey sections 3.1 and 8.2): Codex blocks on the deny
// JSON with exit 0, the answer golaw and the second kit give through every
// shell, Cursor on a deny permission; an allow is no output on Codex and
// the allow JSON on Cursor; any other reply is refused by the row, exit 2
// with a reason included, because powershell -Command turns it into 1, a
// failed hook, so no row may depend on it.
func m4HostBlocks(agent string, got result) (bool, error) {
	if got.err != nil {
		return false, fmt.Errorf("run the hook: %w", got.err)
	}
	if agent == agentCodex {
		switch {
		case got.code == 0 && strings.Contains(got.stdout, `"permissionDecision":"deny"`):
			return true, nil
		case got.code == 0 && got.stdout == "":
			return false, nil
		}
	}
	if agent == agentCursor && got.code == 0 {
		var answer struct {
			Permission string `json:"permission"`
		}
		if json.Unmarshal([]byte(got.stdout), &answer) == nil && (answer.Permission == wantDeny || answer.Permission == wantAllow) {
			return answer.Permission == wantDeny, nil
		}
	}
	return false, fmt.Errorf("a failed hook: exit %d, stdout %q, stderr %q: %w", got.code, got.stdout, got.stderr, ErrAcceptance)
}
