package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/hook/agents"
)

// lineWant is what doctor may say on one line of an agent report right
// after agent install --agent all in the isolated home: the "state code"
// pairs that hold there, and whether the line must be printed.
type lineWant struct {
	codes    []string
	required bool
}

// agentWants are the lines of the Codex and Cursor reports section 10.3
// item 2 judges: every artifact present and matching, the hook naming the
// built binary, Codex trust absent, so never trusted and never protecting,
// Codex failing open as its stated limit, Cursor trusted at user scope,
// failing closed, never protecting, a preview whose writes are not
// enforced and whose capture is unknown.
func agentWants(agent string) map[string]lineWant {
	dispatch := lineWant{[]string{"unknown unobserved", "ok observed", "unknown stale"}, true}
	wants := map[string]lineWant{
		"doctor.installed": {[]string{"ok installed"}, true},
		"doctor.binary":    {[]string{"ok binary-matches"}, true},
		"doctor.dispatch":  dispatch,
		"doctor.duplicate": {[]string{"ok no-duplicate"}, false},
		"doctor.companion": {[]string{"ok companion-absent"}, false},
	}
	if agent == agentCodex {
		wants["doctor.trusted"] = lineWant{[]string{"red untrusted"}, true}
		wants["doctor.state"] = lineWant{[]string{"red installed-untrusted"}, true}
		wants["doctor.fail-mode"] = lineWant{[]string{"unknown host-fails-open"}, true}
		wants["doctor.version"] = lineWant{[]string{
			"ok agent-version-validated", "unknown agent-version-unvalidated", "unknown agent-version-unavailable",
		}, true}
		return wants
	}
	wants["doctor.trusted"] = lineWant{[]string{"ok trusted"}, true}
	wants["doctor.state"] = lineWant{[]string{"unknown trusted-inactive", "unknown preview-active"}, true}
	wants["doctor.fail-mode"] = lineWant{[]string{"ok fail-closed"}, true}
	wants["doctor.preview"] = lineWant{[]string{"unknown cursor-writes-not-enforced"}, true}
	wants["doctor.capture"] = lineWant{[]string{"unknown capture-absent", "unknown capture-recorded"}, true}
	return wants
}

// decodeReports reads doctor --json as the stream of one document per
// agent it prints, each decoded strictly.
func decodeReports(stdout string) ([]doctor.Report, error) {
	decoder := json.NewDecoder(strings.NewReader(stdout))
	decoder.DisallowUnknownFields()
	var reports []doctor.Report
	for {
		var report doctor.Report
		err := decoder.Decode(&report)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode doctor --json: %w", err)
		}
		reports = append(reports, report)
	}
	if len(reports) == 0 {
		return nil, fmt.Errorf("decode doctor --json: no document: %w", ErrAcceptance)
	}
	return reports, nil
}

// judgeAgentReports requires one report per agent of the registry, in its
// order, and judges every report after the Claude one, which judgeDoctor
// holds to M1.
func judgeAgentReports(reports []doctor.Report, built string) error {
	ids := agents.IDs()
	if len(reports) != len(ids) {
		return fmt.Errorf("doctor --json printed %d documents, one per agent wants %d: %w", len(reports), len(ids), ErrAcceptance)
	}
	for i, id := range ids {
		if id == agentClaude {
			continue
		}
		if err := judgeAgentReport(id, reports[i], built); err != nil {
			return err
		}
	}
	return nil
}

// judgeAgentReport holds one report to agentWants: a line outside its
// codes is a contradiction, a required line absent is missing evidence,
// and the placeholder of a doctor not built is never a report.
func judgeAgentReport(agent string, report doctor.Report, built string) error {
	if report.Schema != 1 {
		return fmt.Errorf("the %s report has schema %d: %w", agent, report.Schema, ErrAcceptance)
	}
	wants := agentWants(agent)
	seen := make(map[string]bool, len(wants))
	for _, line := range report.Lines {
		want, judged := wants[line.ID]
		if !judged && line.ID != "doctor.agent" {
			continue
		}
		if err := judgeAgentLine(agent, line, want, built); err != nil {
			return err
		}
		seen[line.ID] = true
	}
	for id, want := range wants {
		if want.required && !seen[id] {
			return fmt.Errorf("the %s report lacks %s: %w", agent, id, ErrAcceptance)
		}
	}
	return nil
}

// judgeAgentLine holds one judged line to its codes, and the binary line
// to the binary this run built.
func judgeAgentLine(agent string, line doctor.Line, want lineWant, built string) error {
	if line.ID == "doctor.agent" {
		return fmt.Errorf("the %s report is the placeholder of a doctor not built: %w", agent, ErrAcceptance)
	}
	if !slices.Contains(want.codes, line.State+" "+line.Code) {
		return fmt.Errorf("doctor %s %s of %s is %s %s (%s): %w", line.ID, line.Key, agent, line.State, line.Code, line.Detail, ErrAcceptance)
	}
	if line.ID == lineBinary {
		return judgeBinary(line, built)
	}
	return nil
}
