package main

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
)

// healthyAgentReport is the report of an agent right after agent install
// --all in the isolated home, the first code agentWants allows on each
// line, the binary line naming the binary that ran doctor.
func healthyAgentReport(agent, binary string) doctor.Report {
	wants := agentWants(agent)
	ids := make([]string, 0, len(wants))
	for id := range wants {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	lines := make([]doctor.Line, 0, len(ids))
	for _, id := range ids {
		state, code, _ := strings.Cut(wants[id].codes[0], " ")
		line := doctor.Line{ID: id, Key: agent + "/user", State: state, Code: code}
		if id == lineBinary {
			line.Detail = binary
		}
		lines = append(lines, line)
	}
	return doctor.Report{Schema: 1, Lines: lines}
}

// agentDocuments are the Codex and Cursor documents doctor prints after
// the Claude one, each after edit.
func agentDocuments(t *testing.T, c command, edit func(string, *doctor.Report)) string {
	t.Helper()
	var out []byte
	for _, agent := range []string{agentCodex, agentCursor} {
		report := healthyAgentReport(agent, c.program)
		if edit != nil {
			edit(agent, &report)
		}
		data, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		out = append(append(out, data...), '\n')
	}
	return string(out)
}

// agentAnswer answers doctor with the healthy Claude report and the agent
// reports after edit, or with stdout itself when it is not empty.
func agentAnswer(t *testing.T, edit func(string, *doctor.Report), stdout string) func(command) result {
	t.Helper()
	return func(c command) result {
		if stdout != "" {
			return result{stdout: stdout}
		}
		claude, err := json.Marshal(healthyReport(c))
		if err != nil {
			t.Fatal(err)
		}
		return result{stdout: string(claude) + "\n" + agentDocuments(t, c, edit)}
	}
}

// setAgentLine changes the line of id in the report of agent.
func setAgentLine(agent, id, state, code string) func(string, *doctor.Report) {
	return func(name string, r *doctor.Report) {
		if name != agent {
			return
		}
		for i := range r.Lines {
			if r.Lines[i].ID == id {
				r.Lines[i].State, r.Lines[i].Code = state, code
			}
		}
	}
}

// dropAgentLine removes the line of id from the report of agent.
func dropAgentLine(agent, id string) func(string, *doctor.Report) {
	return func(name string, r *doctor.Report) {
		if name == agent {
			r.Lines = slices.DeleteFunc(r.Lines, func(l doctor.Line) bool { return l.ID == id })
		}
	}
}

func TestDoctorRowJudgesEveryAgentReport(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	base := s.execute
	other := filepath.Join(s.root, moduleFile)
	for _, tc := range []struct {
		name   string
		edit   func(string, *doctor.Report)
		stdout string
		want   string
	}{
		{"near healthy reports", nil, "", ""},
		{"near a dispatch observed and a capture recorded", func(agent string, r *doctor.Report) {
			setAgentLine(agentCodex, "doctor.dispatch", "ok", "observed")(agent, r)
			setAgentLine(agentCursor, "doctor.capture", "unknown", "capture-recorded")(agent, r)
		}, "", ""},
		{"red the Cursor report missing", nil, `{"schema":1,"lines":[]}`, "lacks a line"},
		{"red Codex reported trusted with no trust record", setAgentLine(agentCodex, "doctor.trusted", "ok", "trusted"), "", "is ok trusted"},
		{"red an untrusted Codex hook reported protecting", setAgentLine(agentCodex, "doctor.state", "ok", "protecting"), "", "is ok protecting"},
		{"red Codex reported fail closed", setAgentLine(agentCodex, "doctor.fail-mode", "ok", "fail-closed"), "", "is ok fail-closed"},
		{"red Cursor reported protecting", setAgentLine(agentCursor, "doctor.state", "ok", "protecting"), "", "is ok protecting"},
		{"red the Cursor preview line absent", dropAgentLine(agentCursor, "doctor.preview"), "", "lacks doctor.preview"},
		{"red a duplicate skill", setAgentLine(agentCursor, "doctor.duplicate", "unknown", "skill-duplicate"), "", "skill-duplicate"},
		{"red the placeholder of a doctor not built", func(agent string, r *doctor.Report) {
			if agent == agentCursor {
				r.Lines = append(r.Lines, doctor.Line{ID: "doctor.agent", Key: agent, State: "unknown", Code: "agent-doctor-not-built"})
			}
		}, "", "placeholder of a doctor not built"},
		{"red the Codex hook naming another binary", func(agent string, r *doctor.Report) {
			if agent == agentCodex {
				r.Lines[slices.IndexFunc(r.Lines, func(l doctor.Line) bool { return l.ID == lineBinary })].Detail = other
			}
		}, "", "not the built"},
		{"red a schema of another version", func(_ string, r *doctor.Report) { r.Schema = 2 }, "", "schema 2"},
	} {
		s.execute = doctorAnswers(base, agentAnswer(t, tc.edit, tc.stdout))
		got := s.doctorRow(t.Context())
		if tc.want == "" && got.status != statusPass || tc.want != "" && (got.status != statusFail || !strings.Contains(got.detail, tc.want)) {
			t.Errorf("%s: %+v", tc.name, got)
		}
	}
}

func TestDecodeReportsReadsOneDocumentPerAgent(t *testing.T) {
	t.Parallel()
	reports, err := decodeReports("{\"schema\":1,\"lines\":[]}\n{\"schema\":1,\"lines\":[]}\n")
	if err != nil || len(reports) != 2 {
		t.Fatalf("reports = %+v, %v", reports, err)
	}
	for name, stdout := range map[string]string{
		"empty": "", "unknown key": `{"schema":1,"lines":[]}{"schema":1,"extra":1}`, "truncated": `{"schema":1,"lines":[]}{`,
	} {
		if _, err := decodeReports(stdout); err == nil || !strings.Contains(err.Error(), "decode doctor --json") {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := judgeAgentReports(reports, "/absent"); err == nil || !strings.Contains(err.Error(), "printed 2 documents, one per agent wants 3") {
		t.Fatalf("two documents = %v", err)
	}
}
