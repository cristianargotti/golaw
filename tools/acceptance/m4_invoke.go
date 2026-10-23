package main

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
)

// m4Reply is the answer one shell gave to a replayed payload.
type m4Reply struct {
	shell string
	got   result
}

// m4Golaw is the one installed handler of an event and a tool whose
// command names the target's binary, held to what its installer writes.
func (s *suite) m4Golaw(t m4Target, agent, event, tool string) (m4Entry, error) {
	entries, err := m4ReadEntries(m4HooksPath(t.home, agent), agent)
	if err != nil {
		return m4Entry{}, err
	}
	var found []m4Entry
	for i := range entries {
		entry := &entries[i]
		if entry.Event != event {
			continue
		}
		selected, selectErr := m4Selects(agent, entry.Matcher, tool)
		if selectErr != nil {
			return m4Entry{}, selectErr
		}
		if selected && m4Names(m4Command(agent, s.m4System(), entry.Handler), t.binary) {
			found = append(found, *entry)
		}
	}
	if len(found) != 1 {
		return m4Entry{}, fmt.Errorf("%s %s %q: %d installed handlers name the binary under test, want one: %w",
			agent, event, tool, len(found), ErrAcceptance)
	}
	return found[0], m4Valid(agent, found[0].Handler)
}

// m4Valid holds a handler to what its installer writes: the timeout 10;
// for Codex a command handler that waits for its answer and carries no
// fail closed field, which Codex does not have (D6, D15); for Cursor
// failClosed true.
func m4Valid(agent string, handler m4Handler) error {
	if handler.Timeout != hookTimeout {
		return fmt.Errorf("the %s handler has the timeout %d, want %d: %w", agent, handler.Timeout, hookTimeout, ErrAcceptance)
	}
	if agent == agentCodex {
		if handler.Type != keyCommand || handler.Async || handler.FailClosed != nil || handler.OnFailure != "" {
			return fmt.Errorf("the codex handler is not a blocking command handler without a fail closed field: %w", ErrAcceptance)
		}
		return nil
	}
	if handler.FailClosed == nil || !*handler.FailClosed {
		return fmt.Errorf("the cursor handler does not set failClosed: %w", ErrAcceptance)
	}
	return nil
}

// m4Shells are the shells a host runs a command string through: sh -c on
// macOS and Linux; on Windows cmd.exe /c and PowerShell with -NoProfile
// -Command, the form Codex uses (section 8.3, survey section 4).
func m4Shells(goos string) [][]string {
	if goos == goosWindows {
		return [][]string{{"cmd.exe", "/c"}, {"powershell.exe", "-NoProfile", "-Command"}}
	}
	return [][]string{{"sh", "-c"}}
}

// m4WorkDir is where the host starts the hook: Codex in the payload's
// cwd, Cursor in ~/.cursor for a user hook (survey section 8.1), a
// directory the decoder must never read as the repository.
func m4WorkDir(agent string, t m4Target) string {
	if agent == agentCursor {
		return filepath.Dir(m4HooksPath(t.home, agentCursor))
	}
	return t.repo
}

// m4Invoke replays a payload through golaw's installed command of the
// event and the tool, under every shell of the system.
func (s *suite) m4Invoke(ctx context.Context, t m4Target, agent, event, tool string, payload []byte) ([]m4Reply, error) {
	entry, err := s.m4Golaw(t, agent, event, tool)
	if err != nil {
		return nil, err
	}
	return s.m4Through(ctx, t, agent, m4Command(agent, s.m4System(), entry.Handler), payload)
}

// m4Through runs one command string under every shell of the system with
// the payload on stdin, closed at its end.
func (s *suite) m4Through(ctx context.Context, t m4Target, agent, line string, payload []byte) ([]m4Reply, error) {
	shells := m4Shells(s.m4System())
	replies := make([]m4Reply, 0, len(shells))
	for _, shell := range shells {
		program, err := s.lookup(shell[0])
		if err != nil {
			return nil, fmt.Errorf("locate %s: %w", shell[0], err)
		}
		args := append(slices.Clone(shell[1:]), line)
		got := s.execute(ctx, command{
			program: program, args: args, dir: m4WorkDir(agent, t), env: t.env, input: string(payload),
		})
		replies = append(replies, m4Reply{shell: shell[0], got: got})
	}
	return replies, nil
}
