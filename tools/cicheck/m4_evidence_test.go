package cicheck_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/agents"
)

func TestM4EvidenceCitations(t *testing.T) {
	t.Parallel()
	const path = "docs/agent-contracts.md"
	text := string(kitBytes(t, path))
	requireText(t, path,
		"M4 evidence review",
		"A missing or skipped row remains documented only.",
		"A dated record applies only to its recorded binary, version and cases.",
		"Source inspection and simulated replies do not establish native execution.",
		"In an offline cell, version= is the candidate version printed by the acceptance report.",
		"In a live cell, version= is the record's agent_version.",
	)
	ids := agents.IDs()
	if len(ids) != 3 {
		t.Fatal("expected three registered agents")
	}
	for _, event := range m4Events() {
		for _, tool := range event.tools {
			for _, system := range []string{"Linux", "macOS", "Windows"} {
				m4EvidenceCitation(t, text, event, tool, system, ids)
			}
		}
	}
}

func m4EvidenceCitation(t *testing.T, text string, event m4Event, tool, system string, ids []string) {
	t.Helper()
	prefix := fmt.Sprintf("| %s | %s | %s | %s | ",
		ids[event.agent], event.event, tool, system)
	level, evidence, found := m4Assurance(text, prefix)
	if !found {
		t.Fatalf("missing assurance row: %s", prefix)
	}
	if level == "documented only" {
		return
	}
	required := []string{"date=", "version=", "binary_sha256="}
	if level == "proven offline" {
		required = append(required, "native", "PASS", "](acceptance/m4.md)")
	}
	if !m4TextOK(evidence, required, nil) {
		t.Fatalf("incomplete evidence: %s", prefix)
	}
	if level == "proven live" {
		m4LiveCitation(t, event, tool, system, ids, evidence)
	}
}

func m4LiveCitation(t *testing.T, event m4Event, tool, system string, ids []string, evidence string) {
	t.Helper()
	if event.agent == 2 || system != "macOS" || event.event != "PreToolUse" {
		t.Fatal("live claim exceeds the recorded M4 scope")
	}
	wantTool := "Write"
	if event.agent == 1 {
		wantTool = "apply_patch"
	}
	if tool != wantTool {
		t.Fatal("live claim names an unrecorded tool")
	}
	relative := "live/" + ids[event.agent] + "-darwin.json"
	if !strings.Contains(evidence, "]("+relative+")") {
		t.Fatal("live claim lacks its exact record link")
	}
	var record map[string]json.RawMessage
	if err := json.Unmarshal(kitBytes(t, "docs/"+relative), &record); err != nil {
		t.Fatal(err)
	}
	for field, want := range map[string]string{"agent": ids[event.agent], "os": "darwin"} {
		if got := m4RecordString(t, record, field); got != want {
			t.Fatalf("%s = %q, want %q", field, got, want)
		}
	}
	for field, label := range map[string]string{
		"date":          "date=",
		"agent_version": "version=",
		"binary_sha256": "binary_sha256=",
	} {
		if !strings.Contains(evidence, label+m4RecordString(t, record, field)) {
			t.Fatalf("evidence disagrees with record field %s", field)
		}
	}
	if event.agent == 1 {
		m4PatchLiveCases(t, record, evidence)
	}
}

func m4RecordString(t *testing.T, record map[string]json.RawMessage, field string) string {
	t.Helper()
	var value string
	if err := json.Unmarshal(record[field], &value); err != nil {
		t.Fatal(err)
	}
	if value == "" {
		t.Fatalf("empty live record field %s", field)
	}
	return value
}

func m4PatchLiveCases(t *testing.T, record map[string]json.RawMessage, evidence string) {
	t.Helper()
	if !strings.Contains(evidence, "denied-write and allowed-control only") {
		t.Fatal("patch live claim does not name its case boundary")
	}
	var cases map[string]map[string]string
	if err := json.Unmarshal(record["cases"], &cases); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][2]string{
		"denied-write":    {"blocked", "absent"},
		"allowed-control": {"allowed", "present"},
	} {
		got := cases[name]
		if got["outcome"] != want[0] || got["target"] != want[1] {
			t.Fatalf("live case %s disagrees with its claim", name)
		}
	}
}

func TestM4AcceptanceRecord(t *testing.T) {
	t.Parallel()
	ids := agents.IDs()
	if len(ids) != 3 {
		t.Fatal("expected three registered agents")
	}
	text := string(kitBytes(t, "docs/acceptance/m4.md"))
	required := []string{
		"# M4 acceptance", "go run ./tools/acceptance -m4",
		"Candidate version", "Candidate binary SHA-256", "Execution location",
		"| Row | Linux | macOS | Windows |",
		"Linux and Windows rows count only from a native run of the m4 job, never from macOS or a cross build.",
		"SKIP never counts as PASS.", "Remote CI or the platform pipeline is the only hard gate.",
		"m4-" + ids[1], "m4-" + ids[2],
		"m4-reason/dash-write", "m4-reason/secret-read", "m4-reason/no-verify",
		"m4-failure/" + ids[1] + "-host-timeout", "m4-failure/" + ids[1] + "-empty-stderr", "m4-failure/slow-path",
		"m4-order/second-kit-before", "m4-order/second-kit-after", "m4-trust/changed-definition",
		"m4-vendor/" + ids[1] + "-execpolicy",
		"live/" + ids[1] + "-denied-write", "live/" + ids[1] + "-allowed-control",
		"[Live record procedure](../live/" + ids[1] + ".md)",
	}
	rows := []struct {
		name, text string
		want       bool
	}{
		{"repository", text, true},
		{"red-missing-native-limit", strings.ReplaceAll(text, required[6], ""), false},
		{"red-missing-digest", strings.ReplaceAll(text, required[3], ""), false},
		{"near-unavailable-run", text + "\nAn unavailable native run remains not run.\n", true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if got := m4TextOK(row.text, required, nil); got != row.want {
				t.Fatalf("acceptance record = %v, want %v", got, row.want)
			}
		})
	}
}
