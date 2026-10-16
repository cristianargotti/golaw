package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
)

// healthyDoctor is the doctor --json of a healthy installation: every line
// the row judges carries its expected code.
func healthyDoctor(t *testing.T) string {
	t.Helper()
	report := doctor.Report{Schema: 1, Lines: []doctor.Line{
		{ID: lineArtifacts, Key: "hooks.json", State: "ok", Code: "artifact-matches"},
		{ID: lineBinary, State: "ok", Code: "binary-matches"},
		{ID: lineHooks, Key: "PreToolUse/Read", State: "ok", Code: "hook-failure-blocked"},
		{ID: lineBuild, State: "source-build", Code: "source-build-unverifiable"},
		{ID: "doctor.trust", State: "unknown", Code: "plugin-hook-trust-unproven"},
	}}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// doctorAnswers answers doctor with stdout and code and every other
// command, the installed hook included, the way the simulated tools do.
func doctorAnswers(base executor, stdout string, code int) executor {
	return func(ctx context.Context, c command) result {
		if len(c.args) != 0 && c.args[0] == "doctor" {
			return result{stdout: stdout, code: code}
		}
		return base(ctx, c)
	}
}

func TestDoctorRow(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	base := s.execute
	s.execute = doctorAnswers(base, healthyDoctor(t), 0)
	assertPass(t, s.doctorRow(t.Context()))
	for name, answer := range map[string]struct {
		stdout string
		code   int
	}{
		"undecodable": {"{", 0},
		"unknown key": {`{"schema":1,"lines":[],"extra":true}`, 0},
		"empty":       {`{"schema":1,"lines":[]}`, 0},
		"failed":      {"", 1},
	} {
		s.execute = doctorAnswers(base, answer.stdout, answer.code)
		if got := s.doctorRow(t.Context()); got.status != statusFail || !strings.Contains(got.detail, answerDetail(name)) {
			t.Errorf("%s: %+v", name, got)
		}
	}
	s.execute = func(context.Context, command) result { return result{stdout: "unexpected"} }
	if got := s.doctorRow(t.Context()); got.status != statusFail || !strings.Contains(got.detail, "does not satisfy") {
		t.Fatalf("%+v", got)
	}
}

// answerDetail is the part of the failure detail each bad answer yields.
func answerDetail(name string) string {
	switch name {
	case "undecodable", "unknown key":
		return "decode doctor --json"
	case "empty":
		return "lacks a line"
	default:
		return "exit 1"
	}
}

func TestJudgeDoctorNamesTheLine(t *testing.T) {
	t.Parallel()
	report := doctor.Report{Schema: 1, Lines: []doctor.Line{
		{ID: lineArtifacts, Key: "hooks.json", State: "red", Code: "artifact-drift"},
	}}
	if err := judgeDoctor(report); err == nil || !strings.Contains(err.Error(), "artifact-drift") {
		t.Fatalf("%v", err)
	}
	report.Lines[0] = doctor.Line{ID: lineBuild, State: "verified", Code: "release-verified"}
	if err := judgeDoctor(report); err == nil || !strings.Contains(err.Error(), lineBuild) {
		t.Fatalf("%v", err)
	}
}
