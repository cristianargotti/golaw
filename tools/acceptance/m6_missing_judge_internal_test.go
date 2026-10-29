package main

import (
	"encoding/json"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
	scanreport "github.com/cristianargotti/golaw/internal/report"
)

func m6Line(tool, code string) doctor.Line {
	return doctor.Line{ID: "doctor.surface-tools", Key: tool, State: "missing", Code: code}
}

func TestM6JudgeDoctorWantsOneLinePerNeededTool(t *testing.T) {
	t.Parallel()
	want := map[string]bool{"ruff": false, "shellcheck": true}
	rows := []struct {
		name  string
		lines []doctor.Line
		pass  bool
	}{
		{"near one line each", []doctor.Line{m6Line("ruff", m6ToolMissing), m6Line("shellcheck", m6ToolAvailable)}, true},
		{"near another doctor line beside", []doctor.Line{m6Line("ruff", m6ToolMissing), m6Line("shellcheck", m6ToolAvailable), {ID: "doctor.surface-languages", Code: "surface-languages"}}, true},
		{"red a tool line twice", []doctor.Line{m6Line("ruff", m6ToolMissing), m6Line("ruff", m6ToolMissing), m6Line("shellcheck", m6ToolAvailable)}, false},
		{"red a needed tool without a line", []doctor.Line{m6Line("ruff", m6ToolMissing)}, false},
		{"red a missing tool called available", []doctor.Line{m6Line("ruff", m6ToolAvailable), m6Line("shellcheck", m6ToolAvailable)}, false},
		{"red a line for a language the repository lacks", []doctor.Line{m6Line("ruff", m6ToolMissing), m6Line("shellcheck", m6ToolAvailable), m6Line("eslint", m6ToolMissing)}, false},
	}
	for _, row := range rows {
		if err := m6JudgeDoctor(row.lines, want); (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
	}
}

func m6PostAnswer(t *testing.T, decision, context string) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"decision": decision, "systemMessage": "aviso",
		"hookSpecificOutput": map[string]string{"hookEventName": eventPost, "additionalContext": context},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(body) + "\n"
}

func TestM6JudgeNoticeAllowsAndHandsTheAgentTheSentence(t *testing.T) {
	t.Parallel()
	notice := "ruff 0.17.2 is not installed: test/deploy/test_extra.py was judged by the native checks only"
	rows := []struct {
		name string
		got  result
		pass bool
	}{
		{"near the notice", result{stdout: m6PostAnswer(t, "", notice+"; run golaw setup --surface, or the pipeline judges the rest.")}, true},
		{"red a block", result{stdout: m6PostAnswer(t, "block", notice)}, false},
		{"red no notice", result{stdout: m6PostAnswer(t, "", "other context")}, false},
		{"red exit 2", result{code: 2, stderr: "golaw: denied\n"}, false},
		{"red not JSON", result{stdout: "{"}, false},
	}
	for _, row := range rows {
		if err := m6JudgeNotice(row.got, notice); (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
	}
}

func TestM6JudgeRungPassesWithEitherSentence(t *testing.T) {
	t.Parallel()
	agent, person := "ruff is not installed", "ruff não está instalado"
	for _, row := range []struct {
		name string
		got  result
		pass bool
	}{
		{"near the person sentence on stderr", result{stderr: person + "\n"}, true},
		{"near the agent sentence on stdout", result{stdout: agent + "\n"}, true},
		{"red the commit refused", result{code: 1, stderr: person + "\n"}, false},
		{"red silent", result{}, false},
	} {
		if err := m6JudgeRung(row.got, agent, person); (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
	}
}

func TestM6JudgeUnmeasuredNamesEveryMissingTool(t *testing.T) {
	t.Parallel()
	statuses := []scanreport.Status{
		{ID: "lint.eslint", Status: m6Unmeasured, Reason: "eslint 10.13.0 not installed: run golaw setup --surface"},
		{ID: "lint.ruff", Status: m6Unmeasured, Reason: "ruff 0.17.2 not installed: run golaw setup --surface"},
	}
	rows := []struct {
		name string
		got  result
		pass bool
	}{
		{"near both unmeasured", result{code: 2, stdout: m6Doc(t, nil, statuses...)}, true},
		{"red exit 0", result{code: 0, stdout: m6Doc(t, nil, statuses...)}, false},
		{"red ruff measured", result{code: 2, stdout: m6Doc(t, nil, statuses[0], scanreport.Status{ID: "lint.ruff", Status: "measured"})}, false},
		{"red the reason names another tool", result{code: 2, stdout: m6Doc(t, nil, statuses[0], scanreport.Status{ID: "lint.ruff", Status: m6Unmeasured, Reason: "a tool is missing"})}, false},
	}
	for _, row := range rows {
		if err := m6JudgeUnmeasured(row.got, []string{"eslint", "ruff"}); (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
	}
	if lines, err := m6DoctorLinesOf(m5DoctorStream(t, m6Line("ruff", m6ToolMissing))); err != nil || len(lines) != 2 {
		t.Fatalf("the doctor stream reads %+v, %v", lines, err)
	}
}
