package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
)

// m4Installer is the simulated golaw installer of the order rows and the
// defects a red case gives it.
type m4Installer struct {
	prepend, appendAlways, dropAll bool
}

// golawEntry is golaw's entry in the event list of an agent's file.
func golawEntry(agent, line string) any {
	if agent == agentCursor {
		return map[string]any{"command": line, "timeout": hookTimeout, "failClosed": true}
	}
	return map[string]any{"matcher": "^(Bash|apply_patch)$", "hooks": []any{
		map[string]any{"type": keyCommand, "command": line, "timeout": hookTimeout},
	}}
}

// edit merges golaw's entries into or out of a list the way the installer
// does, with the defect of the case.
func (m m4Installer) edit(list []any, agent string, entry any, remove bool) []any {
	index := slices.IndexFunc(list, func(item any) bool {
		raw, err := json.Marshal(item)
		return err == nil && strings.Contains(string(raw), " hook "+agent)
	})
	switch {
	case remove && m.dropAll:
		return nil
	case remove && index >= 0:
		return slices.Delete(list, index, index+1)
	case remove:
		return list
	case index >= 0 && !m.appendAlways:
		list[index] = entry
		return list
	case m.prepend:
		return slices.Insert(list, 0, entry)
	}
	return append(list, entry)
}

// answer is golaw agent install or uninstall for one agent in the home of
// the command.
func (m m4Installer) answer(t *testing.T, s *suite, c command) result {
	t.Helper()
	agent, remove := c.args[3], c.args[1] == "uninstall"
	path := m4HooksPath(environmentValue(c.env, "HOME"), agent)
	document := map[string]any{}
	body, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(body, &document)
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	hooks, found := document["hooks"].(map[string]any)
	if !found {
		hooks = map[string]any{}
	}
	entry := golawEntry(agent, install.HookCommand(c.program, s.m4System(), commandHook, agent))
	list, isList := hooks[m4KitEvent(agent)].([]any)
	if !isList {
		list = nil
	}
	hooks[m4KitEvent(agent)], document["hooks"] = m.edit(list, agent, entry, remove), hooks
	out, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	checkedWrite(t, path, string(out))
	return result{}
}

// m4KitShell answers a shell of the order rows: the kit refuses its
// marker, golaw refuses a command that skips the hooks, unless golawAllows.
func m4KitShell(c command, golawAllows bool) result {
	line := c.args[len(c.args)-1]
	cursor := strings.HasSuffix(line, agentCursor)
	blocked := strings.Contains(c.input, m4KitMarker)
	if !strings.Contains(line, m4KitName) {
		blocked = strings.Contains(c.input, "no-verify") && !golawAllows
	}
	switch {
	case cursor && blocked:
		return result{stdout: `{"permission":"deny","user_message":"x","agent_message":"y"}` + "\n"}
	case cursor:
		return result{stdout: `{"permission":"allow"}` + "\n"}
	case blocked:
		return result{stderr: "denied\n", stdout: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"denied"},"systemMessage":"x"}` + "\n"}
	}
	return result{}
}

// m4OrderSuite is a Linux suite whose installer and shells answer as m and
// m4KitShell do.
func m4OrderSuite(t *testing.T, m m4Installer, golawAllows bool) *suite {
	t.Helper()
	s := testSuite(t)
	s.m2OS = "linux"
	base := s.execute
	s.execute = func(ctx context.Context, c command) result {
		switch {
		case len(c.args) == 4 && c.args[0] == "agent":
			return m.answer(t, s, c)
		case c.program == s.git || c.program == s.binary || c.program == s.goTool:
			return base(ctx, c)
		}
		return m4KitShell(c, golawAllows)
	}
	return s
}

func TestM4OrderRowsHoldBothKitsInBothOrders(t *testing.T) {
	t.Parallel()
	s := m4OrderSuite(t, m4Installer{}, false)
	if got := strings.Join(m4RowNames(s.m4OrderRows(t.Context(), false)), ","); got != "second-kit-before PASS,second-kit-after PASS" {
		t.Fatalf("rows = %s", got)
	}
	for _, place := range m4SampleAfterKit(t) {
		if place.kit != 0 || place.golaw != 1 || len(place.raw) == 0 {
			t.Fatalf("a kit added first then golaw: %+v", place)
		}
	}
}

// m4SampleAfterKit installs golaw after a kit written first and reads both
// places.
func m4SampleAfterKit(t *testing.T) map[string]m4Place {
	t.Helper()
	s := m4OrderSuite(t, m4Installer{}, false)
	home, env, err := s.m4Home("sample")
	if err != nil {
		t.Fatal(err)
	}
	target := m4Target{home: home, binary: s.binary, env: env, repo: t.TempDir()}
	if addErr := s.m4AddKits(home, "/opt/kit/"+m4KitName, true); addErr != nil {
		t.Fatal(addErr)
	}
	if installErr := s.m4AgentCommand(t.Context(), target, verbInstall); installErr != nil {
		t.Fatal(installErr)
	}
	places, err := s.m4Places(target, "/opt/kit/"+m4KitName)
	if err != nil {
		t.Fatal(err)
	}
	return places
}

func TestM4OrderRowsFailAnInstallerThatMovesOrDropsAForeignEntry(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, row, want string
		installer       m4Installer
		golawAllows     bool
	}{
		{"red golaw put ahead of a kit installed first", m4KitBefore, "holds the kit at 1 and golaw at 0", m4Installer{prepend: true}, false},
		{"red an upgrade appended beside the old entry", m4KitAfter, "the upgrade moved", m4Installer{appendAlways: true}, false},
		{"red an uninstall that drops the kit", m4KitAfter, "changed the kit entry", m4Installer{dropAll: true}, false},
		{"red golaw letting its own command through", m4KitBefore, "golaw handler", m4Installer{}, true},
	} {
		s := m4OrderSuite(t, tc.installer, tc.golawAllows)
		rows := s.m4OrderRows(t.Context(), false)
		index := slices.IndexFunc(rows, func(item row) bool { return item.name == tc.row })
		if index < 0 || rows[index].status != statusFail || !strings.Contains(rows[index].detail, tc.want) {
			t.Errorf("%s: %+v", tc.name, rows)
		}
	}
	s := m4OrderSuite(t, m4Installer{}, false)
	s.execute = func(context.Context, command) result { return result{code: 1} }
	for _, item := range s.m4OrderRows(t.Context(), false) {
		if item.status != statusFail || !strings.Contains(item.detail, "build m4-kit") {
			t.Errorf("a kit that does not build: %+v", item)
		}
	}
	if blocked, err := m4HostBlocks(agentCodex, result{code: 1}); err == nil || blocked {
		t.Fatal("exit 1 was an answer of the host protocol")
	}
}
