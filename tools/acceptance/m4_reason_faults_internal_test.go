package main

import (
	"context"
	"strings"
	"testing"
)

// m4ReasonFault answers one call of a reason row with a defect instead of
// the reply of m4ReasonScript.
type m4ReasonFault func(c command) (result, bool)

// m4FaultyReasons is a reason suite whose calls fault picks first.
func m4FaultyReasons(t *testing.T, fault m4ReasonFault) *suite {
	t.Helper()
	s := m4ReplaySuite(t)
	script := m4ReasonScript(s, m4SameReasons)
	s.execute = func(ctx context.Context, c command) result {
		if got, faulted := fault(c); faulted {
			return got
		}
		return script(ctx, c)
	}
	return s
}

// m4ReasonDetails are the details of the reason rows by name.
func m4ReasonDetails(ctx context.Context, s *suite) map[string]row {
	out := make(map[string]row)
	for _, item := range s.m4ReasonRows(ctx, false) {
		out[item.name] = item
	}
	return out
}

func TestM4ReasonRowsNameTheAgentThatFailed(t *testing.T) {
	t.Parallel()
	codexShell := func(c command) bool {
		return len(c.args) > 0 && strings.HasSuffix(c.args[len(c.args)-1], "hook codex")
	}
	cursorShell := func(c command) bool {
		return len(c.args) > 0 && strings.HasSuffix(c.args[len(c.args)-1], "hook cursor")
	}
	for _, tc := range []struct {
		name, row, want string
		fault           m4ReasonFault
	}{
		{"red Codex allowing the dash", "dash-write", "codex through sh", func(c command) (result, bool) {
			return result{}, codexShell(c)
		}},
		{"red Codex allowing the bypass", "no-verify", "codex through sh", func(c command) (result, bool) {
			return result{}, codexShell(c)
		}},
		{"red Cursor refusing the write it must capture", "dash-write", "preToolUse through sh", func(c command) (result, bool) {
			return m4CursorReply("", "x"), cursorShell(c) && strings.Contains(c.input, `"preToolUse"`)
		}},
		{"red Cursor quiet after the dash lands", "dash-write", "cursor through sh", func(c command) (result, bool) {
			return result{stdout: "{}\n"}, cursorShell(c) && strings.Contains(c.input, `"postToolUse"`)
		}},
		{"red a Cursor read denial carrying an agent message", "secret-read", "cursor through sh", func(c command) (result, bool) {
			return m4CursorReply("", "x"), cursorShell(c) && strings.Contains(c.input, `"beforeReadFile"`)
		}},
		{"red Claude reading .env", "secret-read", "m4-secret-file through Claude", func(c command) (result, bool) {
			return result{}, len(c.args) == 2 && c.args[0] == commandHook && strings.Contains(c.input, `"Read"`)
		}},
		{"red a Cursor shell reason of its own", "secret-read", "the reasons differ", func(c command) (result, bool) {
			return m4CursorReply(c.input, "another reason"), cursorShell(c) && strings.Contains(c.input, "cat .env")
		}},
		{"red a repository that cannot be made", "no-verify", "exit 1", func(c command) (result, bool) {
			return result{code: 1}, len(c.args) > 0 && c.args[0] == commandInit
		}},
	} {
		got := m4ReasonDetails(t.Context(), m4FaultyReasons(t, tc.fault))[tc.row]
		if got.status != statusFail || !strings.Contains(got.detail, tc.want) {
			t.Errorf("%s: %+v", tc.name, got)
		}
	}
}
