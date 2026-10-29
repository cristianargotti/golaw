package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

func TestM6P95IsTheNineteenthOfTwenty(t *testing.T) {
	t.Parallel()
	samples := make([]time.Duration, 0, m6Runs)
	for i := m6Runs; i >= 1; i-- {
		samples = append(samples, time.Duration(i)*time.Millisecond)
	}
	if got, err := m6P95(samples); err != nil || got != 19*time.Millisecond {
		t.Fatalf("near twenty samples = %s, %v", got, err)
	}
	if got, err := m6P95([]time.Duration{7 * time.Millisecond}); err != nil || got != 7*time.Millisecond {
		t.Fatalf("near one sample = %s, %v", got, err)
	}
	if _, err := m6P95(nil); err == nil {
		t.Fatal("red no sample has a percentile")
	}
}

func TestM6LongFilesHoldThreeHundredLinesOfTheirLanguage(t *testing.T) {
	t.Parallel()
	for language, first := range map[surface.Language]string{
		surface.Python: `"""`, surface.Dockerfile: "FROM ", surface.Shell: "#!/bin/sh",
	} {
		path, body, err := m6Long(language)
		if err != nil || path == "" || bytes.Count(body, []byte("\n")) != m6LongLines || !bytes.HasPrefix(body, []byte(first)) {
			t.Errorf("%s: %s holds %d lines, %v", language, path, bytes.Count(body, []byte("\n")), err)
		}
	}
	if _, _, err := m6Long(surface.CSS); err == nil {
		t.Error("red a language without a hook tool got a long file")
	}
}

func TestM6JudgeKilledWantsTheNoticeBetweenTheKillAndTheEventBudget(t *testing.T) {
	t.Parallel()
	notice := result{stdout: m6PostAnswer(t, "", "ruff: "+m6KilledNotice)}
	rows := []struct {
		name    string
		elapsed time.Duration
		got     result
		pass    bool
	}{
		{"near killed at the budget", 1600 * time.Millisecond, notice, true},
		{"red answered before the kill", 200 * time.Millisecond, notice, false},
		{"red past the event budget", toolchain.HookEventBudget + 2*time.Second, notice, false},
		{"red no notice", 1600 * time.Millisecond, result{stdout: m6PostAnswer(t, "", "clean")}, false},
		{"red a block", 1600 * time.Millisecond, result{stdout: m6PostAnswer(t, "block", m6KilledNotice)}, false},
	}
	for _, row := range rows {
		if err := m6JudgeKilled(row.elapsed, row.got); (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
	}
}

// m6Clock is a clock that moves step on every reading.
func m6Clock(step time.Duration) func() time.Time {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	return func() time.Time {
		now = now.Add(step)
		return now
	}
}

func TestM6BudgetRowsMeasureEveryLanguageAndTheSlowTool(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		row  string
		step time.Duration
		pass bool
	}{
		{"post-p95", 40 * time.Millisecond, true},
		{"post-p95", 2 * time.Second, false},
		{"pre-p95", 40 * time.Millisecond, true},
		{"pre-p95", 400 * time.Millisecond, false},
		{"slow-tool-killed", 1600 * time.Millisecond, true},
		{"slow-tool-killed", 40 * time.Millisecond, false},
	} {
		s := m6TestSuite(t)
		s.shared.now = m6Clock(tc.step)
		var posts int
		s.execute = m5Script(t, s.suite, m6InitReply(),
			m5Reply{prefix: []string{commandHook, agentClaude}, answer: func(t *testing.T, c command) result {
				t.Helper()
				if !strings.Contains(c.input, eventPost) {
					return result{}
				}
				posts++
				return result{stdout: m6PostAnswer(t, "", "ruff: "+m6KilledNotice)}
			}},
		).Execute
		for _, c := range m6BudgetCases() {
			if c.name != tc.row {
				continue
			}
			err := c.run(s.m6Scope(m6BudgetCategory, c.name), t.Context())
			if (err == nil) != tc.pass {
				t.Errorf("%s at %s per reading: %v", tc.row, tc.step, err)
			}
		}
		if tc.row == "post-p95" && tc.pass && posts != 3*(m6Runs+m6Warmups) {
			t.Errorf("post events = %d, want %d", posts, 3*(m6Runs+m6Warmups))
		}
	}
}
