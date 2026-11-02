package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// m7RegenText is the context a candidate gives the port edit: its summary
// and the hole it appended to the owned half of the SQL adapter.
const m7RegenText = m7RegenSummary + ": 3 files, 0 drift\nhole open " + m7OwnedPlans + ":9 " + m7PurgeHole + " PurgePlans: call p.queries.PurgePlans\n"

// m7Regenerated stands in for the regeneration: the stub appended to the
// owned file and the method rendered in the fake.
func m7Regenerated(t *testing.T, c command) {
	t.Helper()
	m2Write(t, filepath.Join(c.dir, filepath.FromSlash(m7OwnedPlans)), "package postgres\n\n// "+m7HoleMarker+m7PurgeHole+" call p.queries.PurgePlans\n")
	m2Write(t, filepath.Join(c.dir, filepath.FromSlash(m7FakePath)), "package ledgertest\n\nfunc (f *Fake) PurgePlans() {}\n")
}

// m7RegenReplies stand in for a candidate that allows every pre event and
// answers every post event with text, regenerating when regen is set.
func m7RegenReplies(t *testing.T, text string, regen bool) []m5Reply {
	t.Helper()
	post := func(t *testing.T, c command) {
		t.Helper()
		if regen {
			m7Regenerated(t, c)
		}
	}
	cursor := 0
	return []m5Reply{
		m6InitReply(),
		{prefix: []string{"new"}, answer: m5After(m7DenialKinds, result{})},
		{prefix: []string{commandHook, agentClaude}, answer: func(t *testing.T, c command) result {
			t.Helper()
			if !strings.Contains(c.input, eventPost) {
				return result{}
			}
			post(t, c)
			if text == "" {
				return result{}
			}
			return result{stdout: m6JSON(t, map[string]any{"hookSpecificOutput": map[string]string{"hookEventName": eventPost, "additionalContext": text}})}
		}},
		{prefix: []string{commandHook, agentCodex}, answer: func(t *testing.T, c command) result {
			t.Helper()
			post(t, c)
			return result{stdout: m6JSON(t, map[string]string{"decision": decisionBlock, "reason": text, "systemMessage": "golaw regenerou."})}
		}},
		{prefix: []string{commandHook, agentCursor}, answer: func(t *testing.T, c command) result {
			t.Helper()
			cursor++
			if cursor%2 == 1 {
				return result{stdout: `{"permission":"allow"}` + "\n"}
			}
			post(t, c)
			return result{stdout: m6JSON(t, map[string]string{"additional_context": text})}
		}},
	}
}

// m7RunRegen runs one m7-regen row by name over the replies and the clock.
func m7RunRegen(t *testing.T, name string, clock func() time.Time, replies []m5Reply) error {
	t.Helper()
	s := newM7Suite(m2TestSuite(t))
	s.shared.now = clock
	s.execute = m5Script(t, s.suite, replies...).Execute
	for _, c := range m7RegenCases() {
		if c.name == name {
			return c.run(s.m7Scope(m7RegenCategory, c.name), t.Context())
		}
	}
	t.Fatalf("no regeneration row %s", name)
	return nil
}

func TestM7RegenRowsPassOverACandidateThatRegenerates(t *testing.T) {
	t.Parallel()
	fast := 40 * time.Millisecond
	for _, name := range []string{"claude-write", "claude-edit", "codex-apply-patch", "claude-bash-heredoc", "cursor-post", "port-p95", "query-p95", "openapi-p95"} {
		if err := m7RunRegen(t, name, m6Clock(fast), m7RegenReplies(t, m7RegenText, true)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if err := m7RunRegen(t, "regenerate-off", m6Clock(fast), m7RegenReplies(t, "", false)); err != nil {
		t.Fatalf("regenerate-off: %v", err)
	}
	notice := "sqlc " + m7NotRegenerated + " (sqlc killed at 1.5s)"
	if err := m7RunRegen(t, "slow-generator-killed", m6Clock(1600*time.Millisecond), m7RegenReplies(t, notice, false)); err != nil {
		t.Fatalf("slow-generator-killed: %v", err)
	}
}

// m7SlowThenFast is a clock slow for the first readings, then fast: the
// measure with the fmt stage over budget, the one without it inside.
func m7SlowThenFast(slow int) func() time.Time {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	readings := 0
	return func() time.Time {
		readings++
		step := time.Millisecond
		if readings <= slow {
			step = 2 * time.Second
		}
		now = now.Add(step)
		return now
	}
}

// TestM7RegenRowsFailOnAMissedOrAFalseRegeneration: a context without the
// summary, a checkout left as it was, a regeneration with the switch off,
// each budget exceeded and a generator that was never killed fail the row.
func TestM7RegenRowsFailOnAMissedOrAFalseRegeneration(t *testing.T) {
	t.Parallel()
	fast, slow := m6Clock(40*time.Millisecond), m6Clock(4*time.Second)
	for _, tc := range []struct {
		name, row, want string
		clock           func() time.Time
		replies         []m5Reply
	}{
		{"red no summary", "claude-write", "lacks", fast, m7RegenReplies(t, "hole open x", true)},
		{"red no context", "claude-edit", "lacks", fast, m7RegenReplies(t, "", true)},
		{"red the checkout unchanged", "codex-apply-patch", "after the regeneration", fast, m7RegenReplies(t, m7RegenText, false)},
		{"red the cursor hole missing", "cursor-post", "lacks the hole", fast, m7RegenReplies(t, m7RegenSummary, true)},
		{"red regenerated with the switch off", "regenerate-off", "hook_regenerate off", fast, m7RegenReplies(t, m7RegenText, false)},
		{"red the owned file grew with the switch off", "regenerate-off", "gained", fast, m7RegenReplies(t, "", true)},
		{"red the port over budget", "port-p95", "the budget is", slow, m7RegenReplies(t, m7RegenText, true)},
		{"red the fmt stage over budget", "port-p95", "A45", m7SlowThenFast(2 * (m6Runs + m6Warmups)), m7RegenReplies(t, m7RegenText, true)},
		{"red the query over budget", "query-p95", "the budget is", slow, m7RegenReplies(t, m7RegenText, true)},
		{"red the generator never killed", "slow-generator-killed", "could be killed", fast, m7RegenReplies(t, m7NotRegenerated, false)},
		{"red the adoption refused", "openapi-p95", "adopt the fixture repository", fast, []m5Reply{{prefix: []string{commandInit}, answer: m5Answer(result{code: 2, stderr: "golaw: refused\n"})}}},
	} {
		err := m7RunRegen(t, tc.row, tc.clock, tc.replies)
		if !errors.Is(err, ErrAcceptance) || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: %v, want an error naming %q", tc.name, err, tc.want)
		}
	}
}

// m7OneHook is a candidate whose hook of agent answers every event with
// answer, after the adoption and golaw new.
func m7OneHook(agent string, answer func(command) result) []m5Reply {
	return []m5Reply{
		m6InitReply(),
		{prefix: []string{"new"}, answer: m5After(m7DenialKinds, result{})},
		{prefix: []string{commandHook, agent}, answer: func(_ *testing.T, c command) result { return answer(c) }},
	}
}

// TestM7RegenShapesStopOnAnAnswerTheyCannotRead: a failed post event, a
// post answer that is no JSON, a Codex post without its block, a denied
// pre event and an adoption the candidate refuses each stop the row.
func TestM7RegenShapesStopOnAnAnswerTheyCannotRead(t *testing.T) {
	t.Parallel()
	deny := func(c command) result {
		if strings.Contains(c.input, eventPost) {
			return result{}
		}
		return result{stdout: `{"systemMessage":"bloqueado","hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"denied"}}` + "\n"}
	}
	refused := []m5Reply{{prefix: []string{commandInit}, answer: m5Answer(result{code: 2, stderr: "golaw: refused\n"})}}
	for _, tc := range []struct {
		name, row, want string
		replies         []m5Reply
	}{
		{"red the post failed", "claude-write", "answered 2", m7OneHook(agentClaude, func(command) result { return result{code: 2, stderr: "golaw: failed\n"} })},
		{"red the post unreadable", "claude-edit", "decode the Claude answer", m7OneHook(agentClaude, func(command) result { return result{stdout: "not json\n"} })},
		{"red codex without its block", "codex-apply-patch", "apply_patch post event", m7OneHook(agentCodex, func(command) result { return result{} })},
		{"red the heredoc denied", "claude-bash-heredoc", "does not satisfy", m7OneHook(agentClaude, deny)},
		{"red the switched edit denied", "regenerate-off", "does not satisfy", m7OneHook(agentClaude, deny)},
		{"red the switch adoption refused", "regenerate-off", "adopt the fixture repository", refused},
		{"red the cursor adoption refused", "cursor-post", "adopt the fixture repository", refused},
		{"red the port adoption refused", "port-p95", "adopt the fixture repository", refused},
		{"red the slow adoption refused", "slow-generator-killed", "adopt the fixture repository", refused},
	} {
		err := m7RunRegen(t, tc.row, m6Clock(40*time.Millisecond), tc.replies)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: %v, want an error naming %q", tc.name, err, tc.want)
		}
	}
}
