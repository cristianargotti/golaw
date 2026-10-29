package main

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	scanreport "github.com/cristianargotti/golaw/internal/report"
	"github.com/cristianargotti/golaw/internal/surfacefix"
)

func TestM6JudgeRedHoldsEveryCountOfItsSource(t *testing.T) {
	t.Parallel()
	exps := surfacefix.Expectations()
	native, lint := m6Found(exps, false), m6Found(exps, true)
	extra := append(append([]model.Finding{}, native...), model.Finding{Check: "lint.eslint", Path: "views/panel.html", Code: "curly", Severity: model.SeverityFail})
	rows := []struct {
		name string
		got  result
		lint bool
		pass bool
	}{
		{"near native counts", result{code: 1, stdout: m6Doc(t, native)}, false, true},
		{"near lint counts", result{code: 1, stdout: m6Doc(t, lint)}, true, true},
		{"near a finding no class claims", result{code: 1, stdout: m6Doc(t, extra)}, false, true},
		{"red one native finding short", result{code: 1, stdout: m6Doc(t, native[1:])}, false, false},
		{"red one lint finding more", result{code: 1, stdout: m6Doc(t, append(lint, lint[0]))}, true, false},
		{"red exit clean", result{code: 0, stdout: m6Doc(t, native)}, false, false},
		{"red two documents", result{code: 1, stdout: m6Doc(t, native) + m6Doc(t, native)}, false, false},
		{"red not JSON", result{code: 1, stdout: "{"}, false, false},
		{"red the process failed", result{err: ErrAcceptance}, false, false},
	}
	for _, row := range rows {
		if err := m6JudgeRed(row.got, exps, row.lint); (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
	}
}

func TestM6JudgeCleanWantsZeroFindingsAndNothingUnmeasured(t *testing.T) {
	t.Parallel()
	unmeasured := scanreport.Status{ID: "lint.eslint", Status: "unmeasured", Reason: "eslint 10.13.0 not installed"}
	note := []model.Finding{{Check: "surface.html-prefer-tag", Path: "views/panel.html", Code: "prefer-tag", Severity: model.SeverityNote}}
	rows := []struct {
		name string
		got  result
		pass bool
	}{
		{"near clean", result{stdout: m6Doc(t, nil)}, true},
		{"red a note", result{stdout: m6Doc(t, note)}, false},
		{"red exit 1", result{code: 1, stdout: m6Doc(t, nil)}, false},
		{"red exit 2 unmeasured", result{code: 2, stdout: m6Doc(t, nil, unmeasured)}, false},
		{"red not JSON", result{stdout: "verdict clean"}, false},
	}
	for _, row := range rows {
		err := m6JudgeClean(row.got)
		if (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
		if err != nil && row.name == "red a note" && !strings.Contains(err.Error(), "views/panel.html") {
			t.Errorf("the refusal does not name the finding: %v", err)
		}
	}
	if got := m6Want(surfacefix.Expectations(), true)[m6Key{"scripts/smoke.sh", "lint.shellcheck", "sc2086"}]; got != 1 {
		t.Fatalf("the lint expectations count sc2086 %d times", got)
	}
}

func TestM6JudgeTwinPicksTheJudgeOfTheTwin(t *testing.T) {
	t.Parallel()
	red := result{code: 1, stdout: m6Doc(t, m6Found(surfacefix.Expectations(), true))}
	if err := m6JudgeTwin(red, surfacefix.Red, true); err != nil {
		t.Fatalf("near the red twin with its lint counts: %v", err)
	}
	if err := m6JudgeTwin(red, surfacefix.Corrected, true); err == nil {
		t.Fatal("red the corrected twin passed with findings")
	}
	if err := m6JudgeTwin(result{stdout: m6Doc(t, nil)}, surfacefix.Corrected, false); err != nil {
		t.Fatalf("near the corrected twin clean: %v", err)
	}
}
