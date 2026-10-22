package cicheck_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/agents"
)

type m4Event struct {
	agent int
	event string
	tools []string
}

func m4Events() []m4Event {
	return []m4Event{
		{0, "PreToolUse", []string{"Write", "Edit", "NotebookEdit"}},
		{0, "PreToolUse", []string{"Read"}},
		{0, "PreToolUse", []string{"Bash", "PowerShell"}},
		{0, "PostToolUse", []string{"Write", "Edit", "NotebookEdit"}},
		{0, "PostToolUse", []string{"Bash", "PowerShell"}},
		{0, "PostToolUseFailure", []string{"Bash", "PowerShell"}},
		{1, "PreToolUse", []string{"apply_patch", "Bash"}},
		{1, "PostToolUse", []string{"apply_patch", "Bash"}},
		{2, "preToolUse", []string{"Write", "Delete", "Shell"}},
		{2, "beforeReadFile", []string{"Read"}},
		{2, "beforeShellExecution", []string{"Shell"}},
		{2, "postToolUse", []string{"Write", "Shell"}},
		{2, "postToolUseFailure", []string{"Write", "Shell"}},
	}
}

func m4Assurance(text, prefix string) (string, string, bool) {
	count := 0
	found := ""
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			count++
			found = strings.TrimPrefix(line, prefix)
		}
	}
	if count != 1 {
		return "", "", false
	}
	cells := strings.Split(strings.TrimSuffix(found, " |"), " | ")
	if len(cells) != 2 || cells[1] == "" {
		return "", "", false
	}
	return cells[0], cells[1], true
}

func TestM4AssuranceMatrix(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, "docs/agent-contracts.md"))
	ids := agents.IDs()
	if len(ids) != 3 {
		t.Fatal("expected three registered agents")
	}
	for _, event := range m4Events() {
		for _, tool := range event.tools {
			for _, system := range []string{"Linux", "macOS", "Windows"} {
				m4CheckAssurance(t, text, event, tool, system, ids)
			}
		}
	}
}

func m4CheckAssurance(t *testing.T, text string, event m4Event, tool, system string, ids []string) {
	t.Helper()
	prefix := fmt.Sprintf("| %s | %s | %s | %s | ",
		ids[event.agent], event.event, tool, system)
	level, evidence, found := m4Assurance(text, prefix)
	if !found {
		t.Errorf("missing or repeated assurance row: %s", prefix)
		return
	}
	switch level {
	case "documented only", "proven offline":
	case "proven live":
		if event.agent == 2 {
			t.Errorf("preview has a live claim: %s", prefix)
		}
	default:
		t.Errorf("unknown assurance %q: %s", level, prefix)
	}
	if strings.TrimSpace(evidence) == "" {
		t.Errorf("empty evidence: %s", prefix)
	}
}

func TestM4AssuranceRowPairs(t *testing.T) {
	t.Parallel()
	prefix := "| probe | PreToolUse | Write | Linux | "
	row := prefix + "documented only | Contract only. |"
	rows := []struct {
		name, text string
		want       bool
	}{
		{"red-missing", "", false},
		{"red-duplicate", row + "\n" + row, false},
		{"red-no-evidence", prefix + "documented only |  |", false},
		{"near-single", row, true},
		{"near-other-event", row + "\n| probe | PostToolUse | Write | Linux | documented only | Contract only. |", true},
	}
	for _, item := range rows {
		t.Run(item.name, func(t *testing.T) {
			t.Parallel()
			_, _, got := m4Assurance(item.text, prefix)
			if got != item.want {
				t.Fatalf("complete row = %v, want %v", got, item.want)
			}
		})
	}
}

func TestM4WireContract(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, "docs/agent-contracts.md"))
	ids := agents.IDs()
	if len(ids) != 3 {
		t.Fatal("expected three registered agents")
	}
	patch, preview := m4AgentName(ids[1]), m4AgentName(ids[2])
	required := []string{
		"2.1.295", "2.1.299", "2.1.306", "0.159.3", "0.163.0",
		"Installed version is not validation evidence.",
		"The " + patch + " patch grammar is pinned to 0.163.0.",
		"The English reason comes from hook.Result.Reason without rewriting.",
		preview + " beforeReadFile has no documented agent_message field.",
		"Empty stdout with exit 0 is the normal " + patch + " allow.",
		"Every " + patch + " exit 2 answer has a non-empty English stderr line.",
		"Every " + patch + " pre denial exits 0 with the deny JSON, the answer that blocks through every shell.",
		"On exit 2 " + patch + " reads stderr, not the deny JSON or systemMessage.",
		"Every " + preview + " reply with exit 0 is a JSON document.",
		preview + " Write and Delete stay capture and allow.",
		"preToolUse Shell and beforeShellExecution both judge the command, any denial winning; preToolUse Shell also keeps the baseline.",
		"generation_id is a turn identity.",
		"Unknown payloads receive the agent's cannot-judge answer.",
		"Offline acceptance does not prove host dispatch, trust or host timeout behavior.",
		"SKIP never counts as PASS.",
		preview + " captures a Write or Delete tool_input that is no object by its JSON type and allows it.",
		patch + " is supported only with a dated live record of macOS and the m4 job green on Linux, macOS and Windows; until both exist, " + patch + " is not yet supported.",
	}
	rows := []struct {
		name, text string
		want       bool
	}{
		{"repository", text, true},
		{"red-missing-wire-limit", strings.ReplaceAll(text, required[7], ""), false},
		{"near-historical-version", text + "\nPrevious version evidence remains historical.\n", true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if got := m4TextOK(row.text, required, nil); got != row.want {
				t.Fatalf("wire contract = %v, want %v", got, row.want)
			}
		})
	}
}
