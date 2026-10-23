package main

import (
	"context"
	"strings"
	"testing"
)

// m4CursorReply is the Cursor answer of a reason row: the Write captured
// and allowed before the tool, the finding after it, the read refused
// without an agent message and the shell refused with one.
func m4CursorReply(input, reason string) result {
	switch {
	case strings.Contains(input, `"postToolUse"`):
		return result{stdout: `{"additional_context":"` + reason + `"}` + "\n"}
	case strings.Contains(input, `"preToolUse"`):
		return result{stdout: `{"permission":"allow"}` + "\n"}
	case strings.Contains(input, `"beforeReadFile"`):
		return result{stdout: `{"permission":"deny","user_message":"` + sampleHuman + `"}` + "\n"}
	}
	return result{stdout: `{"permission":"deny","user_message":"` + sampleHuman + `","agent_message":"` + reason + `"}` + "\n"}
}

// m4ReasonScript answers the three agents of a reason row, the English
// reason of each picked by reason and the human line the sample one.
func m4ReasonScript(s *suite, reason func(agent, input string) string) executor {
	base := s.execute
	return func(ctx context.Context, c command) result {
		deny := func(r string) string {
			return `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"` +
				r + `"},"systemMessage":"` + sampleHuman + `"}`
		}
		switch {
		case c.program == s.binary && len(c.args) == 2 && c.args[0] == commandHook:
			return result{stdout: deny(reason(agentClaude, c.input))}
		case c.program == s.git || c.program == s.binary || c.program == s.goTool:
			return base(ctx, c)
		case strings.HasSuffix(c.args[len(c.args)-1], "hook codex"):
			r := reason(agentCodex, c.input)
			return result{stderr: r + "\n", stdout: deny(r) + "\n"}
		}
		return m4CursorReply(c.input, reason(agentCursor, c.input))
	}
}

// m4SameReasons gives every agent the reason of the event the input
// names.
func m4SameReasons(_, input string) string {
	switch {
	case strings.Contains(input, ".env"):
		return "paths.secret-material: a secret file"
	case strings.Contains(input, "no-verify"):
		return "shell.no-verify: the command skips the git hooks"
	}
	return "text.dashes: a typographic dash (note.txt:1)"
}

func TestM4ReasonRowsCompareTheThreeAgents(t *testing.T) {
	t.Parallel()
	s := m4ReplaySuite(t)
	s.execute = m4ReasonScript(s, m4SameReasons)
	rows := s.m4ReasonRows(t.Context(), false)
	if got := m4RowNames(rows); strings.Join(got, ",") != "dash-write PASS,secret-read PASS,no-verify PASS" {
		t.Fatalf("rows = %+v", rows)
	}
	for name, reason := range map[string]func(string, string) string{
		"dash-write": func(agent, input string) string {
			if agent == agentCursor && strings.Contains(input, `"postToolUse"`) {
				return "text.dashes: a typographic dash (note.txt)"
			}
			return m4SameReasons(agent, input)
		},
		"no-verify": func(agent, input string) string {
			if agent == agentCodex && strings.Contains(input, "no-verify") {
				return "shell.no-verify: skipped hooks"
			}
			return m4SameReasons(agent, input)
		},
	} {
		s.execute = m4ReasonScript(s, reason)
		for _, item := range s.m4ReasonRows(t.Context(), false) {
			if item.name == name && (item.status != statusFail || !strings.Contains(item.detail, "the reasons differ")) {
				t.Errorf("%s with one reason changed: %+v", name, item)
			}
		}
	}
}

func TestM4ReasonRowsFailWhenAnAgentLetsTheEventThrough(t *testing.T) {
	t.Parallel()
	s := m4ReplaySuite(t)
	script := m4ReasonScript(s, m4SameReasons)
	s.execute = func(ctx context.Context, c command) result {
		if c.program == s.binary && len(c.args) == 2 && c.args[0] == commandHook {
			return result{}
		}
		return script(ctx, c)
	}
	for _, item := range s.m4ReasonRows(t.Context(), false) {
		if item.status != statusFail || !strings.Contains(item.detail, "through Claude") {
			t.Errorf("Claude allowing every event: %+v", item)
		}
	}
	if err := m4SameText("reasons", map[string]string{agentClaude: "", agentCodex: ""}); err == nil {
		t.Fatal("two empty reasons were the same reason")
	}
	if _, err := s.m4Sample(agentCodex, "absent.json", s.base, nil); err == nil {
		t.Fatal("an absent sample passed")
	}
}
