package main

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
	scanreport "github.com/cristianargotti/golaw/internal/report"
	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/surfacefix"
)

// m6MissingReplies stand in for a candidate on a machine without surface
// tools: doctor names every tool m6Needed finds missing, the Claude post
// event carries every notice of the hook files, the armed rung prints the
// person sentence, and lint leaves every missing tool unmeasured.
func m6MissingReplies(t *testing.T) []m5Reply {
	t.Helper()
	files := m6HookFiles()
	notices, people := make([]string, 0, len(files)), make([]string, 0, len(files))
	for _, f := range files {
		agent, person, err := m6Notice(f.tool, f.path)
		if err != nil {
			t.Fatal(err)
		}
		notices, people = append(notices, agent), append(people, person)
	}
	needed := func(c command) map[string]bool {
		want, err := m6Needed(t.Context(), c.dir, c.env)
		if err != nil {
			t.Fatal(err)
		}
		return want
	}
	return []m5Reply{
		m6InitReply(),
		{prefix: []string{"setup"}, answer: m5Answer(result{})},
		{prefix: []string{m5DoctorCommand, m2JSONFlag}, answer: func(t *testing.T, c command) result {
			t.Helper()
			names := slices.Sorted(maps.Keys(needed(c)))
			lines := make([]doctor.Line, 0, len(names))
			for _, name := range names {
				lines = append(lines, doctor.Line{ID: "doctor.surface-tools", Key: name, State: "missing", Code: m6ToolMissing})
			}
			return result{stdout: m5DoctorStream(t, lines...)}
		}},
		{prefix: []string{commandHook, agentClaude}, answer: func(t *testing.T, c command) result {
			t.Helper()
			if !strings.Contains(c.input, eventPost) {
				return result{}
			}
			return result{stdout: m6PostAnswer(t, "", strings.Join(notices, "\n"))}
		}},
		{prefix: []string{commandHook, rung.PreCommit}, answer: m5Answer(result{stderr: strings.Join(people, "\n") + "\n"})},
		{prefix: []string{m6LintCommand, m6SurfaceFlag, m2JSONFlag}, answer: func(t *testing.T, c command) result {
			t.Helper()
			names := slices.Sorted(maps.Keys(needed(c)))
			statuses := make([]scanreport.Status, 0, len(names))
			for _, name := range names {
				statuses = append(statuses, scanreport.Status{ID: "lint." + name, Status: m6Unmeasured, Reason: name + " not installed"})
			}
			return result{code: 2, stdout: m6Doc(t, nil, statuses...)}
		}},
	}
}

func TestM6MissingRowsPassOverACandidateWithoutTools(t *testing.T) {
	t.Parallel()
	for _, c := range m6MissingCases() {
		s := m6TestSuite(t)
		s.execute = m5Script(t, s.suite, m6MissingReplies(t)...).Execute
		if err := c.run(s.m6Scope(m6MissingCategory, c.name), t.Context()); err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}

func TestM6MissingRowsRefuseADenial(t *testing.T) {
	t.Parallel()
	deny, err := json.Marshal(map[string]any{
		"systemMessage": "bloqueado",
		"hookSpecificOutput": map[string]string{
			"hookEventName": eventPre, "permissionDecision": wantDeny, "permissionDecisionReason": "denied",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := m6TestSuite(t)
	replies := append([]m5Reply{{prefix: []string{commandHook, agentClaude}, answer: m5Answer(result{stdout: string(deny) + "\n"})}}, m6MissingReplies(t)...)
	s.execute = m5Script(t, s.suite, replies...).Execute
	if err := m6MissingCases()[1].run(s.m6Scope(m6MissingCategory, "hook-denied"), t.Context()); err == nil {
		t.Fatal("a pre event denied on a missing tool and the row passed")
	}
}

func TestM6NeededNamesTheToolsOfTheTwinLanguages(t *testing.T) {
	t.Parallel()
	s := m6TestSuite(t)
	s.execute = m5Script(t, s.suite, m6InitReply()).Execute
	repo, err := s.m6Twin(t.Context(), "needed", surfacefix.Red, nil)
	if err != nil {
		t.Fatal(err)
	}
	want, err := m6Needed(t.Context(), repo, s.env)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"eslint", "ruff", "hadolint", "shellcheck", "checkov"} {
		if usable, found := want[name]; !found || usable {
			t.Errorf("%s: needed %v, usable %v on a PATH without tools", name, found, usable)
		}
	}
}
