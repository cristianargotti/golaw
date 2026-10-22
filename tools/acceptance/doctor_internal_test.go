package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
)

// shippedHooks are the event and matcher keys of the hook definition the
// plugin ships, the hook lines doctor owes the simulated receipt.
func shippedHooks() []string {
	return []string{
		"PostToolUse/Bash|PowerShell", "PostToolUse/Write|Edit|NotebookEdit", "PostToolUseFailure/Bash|PowerShell",
		"PreToolUse/Bash|PowerShell", "PreToolUse/Read", "PreToolUse/Write|Edit|NotebookEdit",
	}
}

// healthyReport is the doctor --json of a healthy simulated installation
// for the command that asks for it: a matching line for each artifact of
// the receipt the simulated installer wrote under its home and for each
// shipped hook, and the binary the command ran.
func healthyReport(c command) doctor.Report {
	artifacts := simulatedArtifacts(environmentValue(c.env, "HOME"))
	lines := make([]doctor.Line, 0, len(artifacts)+len(shippedHooks())+3)
	for _, artifact := range artifacts {
		lines = append(lines, doctor.Line{ID: lineArtifacts, Key: artifact, State: "ok", Code: "artifact-matches"})
	}
	lines = append(lines, doctor.Line{ID: lineBinary, State: "ok", Code: "binary-matches", Detail: c.program})
	for _, key := range shippedHooks() {
		lines = append(lines, doctor.Line{ID: lineHooks, Key: key, State: "ok", Code: "hook-failure-blocked"})
	}
	lines = append(lines,
		doctor.Line{ID: lineBuild, State: "source-build", Code: "source-build-unverifiable"},
		doctor.Line{ID: "doctor.trust", State: "unknown", Code: "plugin-hook-trust-unproven"},
		doctor.Line{ID: lineDispatch, State: "ok", Code: "dispatch-observed"},
	)
	return doctor.Report{Schema: 1, Lines: lines}
}

// reportAnswer answers doctor with the healthy Claude report after edit,
// followed by the healthy reports of the other agents, one document each.
func reportAnswer(t *testing.T, edit func(*doctor.Report)) func(command) result {
	t.Helper()
	return func(c command) result {
		report := healthyReport(c)
		if edit != nil {
			edit(&report)
		}
		data, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		return result{stdout: string(data) + "\n" + agentDocuments(t, c, nil)}
	}
}

// fixedAnswer answers doctor with stdout and code whatever it asked.
func fixedAnswer(stdout string, code int) func(command) result {
	return func(command) result { return result{stdout: stdout, code: code} }
}

// doctorAnswers answers doctor through answer and every other command, the
// installed hook included, the way the simulated tools do.
func doctorAnswers(base executor, answer func(command) result) executor {
	return func(ctx context.Context, c command) result {
		if len(c.args) != 0 && c.args[0] == "doctor" {
			return answer(c)
		}
		return base(ctx, c)
	}
}

// editLine changes the first line of an id.
func editLine(id string, edit func(*doctor.Line)) func(*doctor.Report) {
	return func(r *doctor.Report) {
		edit(&r.Lines[slices.IndexFunc(r.Lines, func(l doctor.Line) bool { return l.ID == id })])
	}
}

// dropLine removes the first line of an id.
func dropLine(id string) func(*doctor.Report) {
	return func(r *doctor.Report) {
		i := slices.IndexFunc(r.Lines, func(l doctor.Line) bool { return l.ID == id })
		r.Lines = slices.Delete(r.Lines, i, i+1)
	}
}

// badDoctorAnswers are the answers the row must refuse, each with the part
// of the failure detail it yields.
func badDoctorAnswers(t *testing.T, s *suite) map[string]struct {
	answer func(command) result
	detail string
} {
	t.Helper()
	other := filepath.Join(s.root, moduleFile)
	return map[string]struct {
		answer func(command) result
		detail string
	}{
		"undecodable":      {fixedAnswer("{", 0), "decode doctor --json"},
		"unknown key":      {fixedAnswer(`{"schema":1,"lines":[],"extra":true}`, 0), "decode doctor --json"},
		"empty":            {fixedAnswer(`{"schema":1,"lines":[]}`, 0), "lacks a line"},
		"failed":           {fixedAnswer("", 1), "exit 1"},
		"schema":           {reportAnswer(t, func(r *doctor.Report) { r.Schema = 2 }), "schema is 2"},
		"omitted artifact": {reportAnswer(t, dropLine(lineArtifacts)), "the receipt lists"},
		"omitted hook":     {reportAnswer(t, dropLine(lineHooks)), "the receipt lists"},
		"extra hook": {reportAnswer(t, func(r *doctor.Report) {
			r.Lines = append(r.Lines, doctor.Line{ID: lineHooks, Key: "PreToolUse/Glob", State: "ok", Code: "hook-failure-blocked"})
		}), "the receipt lists"},
		"wrong binary":        {reportAnswer(t, editLine(lineBinary, func(l *doctor.Line) { l.Detail = other })), "not the built"},
		"unnamed binary":      {reportAnswer(t, editLine(lineBinary, func(l *doctor.Line) { l.Detail = "" })), "binary line names"},
		"contradictory state": {reportAnswer(t, editLine(lineBuild, func(l *doctor.Line) { l.State = "verified" })), "is verified source-build-unverifiable"},
		"drifted artifact": {reportAnswer(t, editLine(lineArtifacts, func(l *doctor.Line) {
			l.State, l.Code = "red", "artifact-drift"
		})), "artifact-drift"},
		"repeated build": {reportAnswer(t, func(r *doctor.Report) {
			r.Lines = append(r.Lines, doctor.Line{ID: lineBuild, State: "source-build", Code: "source-build-unverifiable"})
		}), "repeats the binary or the build line"},
		"undispatched": {reportAnswer(t, editLine(lineDispatch, func(l *doctor.Line) {
			l.State, l.Code = "unknown", "dispatch-unobserved"
		})), "dispatch-unobserved"},
	}
}

func TestDoctorRow(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	base := s.execute
	s.execute = doctorAnswers(base, reportAnswer(t, nil))
	assertPass(t, s.doctorRow(t.Context()))
	for name, bad := range badDoctorAnswers(t, s) {
		s.execute = doctorAnswers(base, bad.answer)
		if got := s.doctorRow(t.Context()); got.status != statusFail || !strings.Contains(got.detail, bad.detail) {
			t.Errorf("%s: %+v", name, got)
		}
	}
	s.execute = func(context.Context, command) result { return result{stdout: "unexpected"} }
	if got := s.doctorRow(t.Context()); got.status != statusFail || !strings.Contains(got.detail, "does not satisfy") {
		t.Fatalf("%+v", got)
	}
}

func TestJudgeBinaryAndReceiptSearchFailures(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	checkedWrite(t, file, "x")
	if err := judgeBinary(doctor.Line{Detail: file}, filepath.Join(dir, "absent")); err == nil || !strings.Contains(err.Error(), "inspect the built binary") {
		t.Fatalf("%v", err)
	}
	if _, err := findReceipt(filepath.Join(dir, "absent")); err == nil || !strings.Contains(err.Error(), "find the receipt") {
		t.Fatalf("%v", err)
	}
}

func TestDoctorRowReadsTheOneReceipt(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	s.execute = doctorAnswers(s.execute, reportAnswer(t, nil))
	receipt := filepath.Join(s.home, "state", "receipt.json")
	body, err := readFile(receipt)
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		path, body, detail string
	}{
		"second receipt":   {filepath.Join(s.home, "state", "other", "receipt.json"), string(body), "found 2 receipts"},
		"undecodable":      {receipt, "{", "decode the receipt"},
		"without artifact": {receipt, `{"schema":1,"artifacts":[],"hook_definition":{"hooks":{}}}`, "lists no artifact"},
	} {
		checkedWrite(t, tc.path, tc.body)
		if got := s.doctorRow(t.Context()); got.status != statusFail || !strings.Contains(got.detail, tc.detail) {
			t.Errorf("%s: %+v", name, got)
		}
		if tc.path != receipt {
			if err := os.Remove(tc.path); err != nil {
				t.Fatal(err)
			}
		}
		checkedWrite(t, receipt, string(body))
	}
	if err := os.Remove(receipt); err != nil {
		t.Fatal(err)
	}
	if got := s.doctorRow(t.Context()); got.status != statusFail || !strings.Contains(got.detail, "found 0 receipts") {
		t.Fatalf("%+v", got)
	}
}
