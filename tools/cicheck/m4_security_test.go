package cicheck_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/agents"
)

func m4AgentName(id string) string {
	return strings.ToUpper(id[:1]) + id[1:]
}

func m4TextOK(text string, required, forbidden []string) bool {
	for _, fragment := range required {
		if !strings.Contains(text, fragment) {
			return false
		}
	}
	for _, fragment := range forbidden {
		if strings.Contains(text, fragment) {
			return false
		}
	}
	return true
}

func TestM4SecurityResiduals(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, "docs/security.md"))
	ids := agents.IDs()
	if len(ids) != 3 {
		t.Fatal("expected three registered agents")
	}
	patch, preview := m4AgentName(ids[1]), m4AgentName(ids[2])
	required := []string{
		patch + " has no read event: the rule runs on Bash tokens and optional MCP read matchers.",
		"Optional MCP matchers do not establish general MCP coverage.",
		patch + " fails open on host failures and skips untrusted hooks silently.",
		"The internal deadline, prefix rules, post feedback, optional staged scanning and doctor reduce these gaps.",
		"The internal budget is 8 seconds; the installed host timeout is 10 seconds.",
		"A user-scope rules file also applies outside enrolled repositories.",
		"Prefix rules cannot express an option anywhere, git -C or arbitrary wrappers.",
		"A malformed rules file can discard all user and project rules for the session.",
		"The workdir argument is absent from the hook payload.",
		"An intercepted patch has only the Bash pre event.",
		"A failed multi-file patch can leave earlier files written without a post event.",
		"A running command can receive its post event later, or never.",
		preview + "'s beforeReadFile prevents exposure to the model, not the disk read.",
		"Write and Delete capture records key names, never values or content.",
		"Post feedback does not undo a write.",
		"Remote CI or the platform pipeline is the only hard gate.",
		patch + " stops a dash write and an oversized file before it exists through apply_patch when the whole candidate can be reconstructed, a secret read through Bash tokens, and a --no-verify commit.",
		patch + " cannot stop write_stdin into an interactive interpreter, hosted and MCP tools beyond the optional matchers, an untrusted or failed hook, or background writers.",
		preview + " stops a secret read by name at beforeReadFile, for exposure to the model only, and the shell denials at beforeShellExecution.",
		preview + " cannot stop the disk read, and cannot stop writes until a live record proves the Write payload (A4, cut 13).",
		"Assurance levels for each agent and OS are recorded in the [agent contract matrix](agent-contracts.md).",
		"an untrusted project layer",
		"On Windows a process that " + patch + " app-server starts may outlive doctor; on macOS and Linux doctor ends its whole process group.",
	}
	forbidden := []string{
		"All agents fail closed on host failures.",
		"Installation proves active protection.",
		"Read hooks prevent every disk read.",
	}
	rows := []struct {
		name, text string
		want       bool
	}{
		{"repository", text, true},
		{"red-missing-read-limit", strings.ReplaceAll(text, required[0], ""), false},
		{"red-host-overclaim", text + "\n" + forbidden[0], false},
		{"red-install-overclaim", text + "\n" + forbidden[1], false},
		{"red-read-overclaim", text + "\n" + forbidden[2], false},
		{"near-extra-guidance", text + "\nA source survey is not a live run.\n", true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if got := m4TextOK(row.text, required, forbidden); got != row.want {
				t.Fatalf("documentation contract = %v, want %v", got, row.want)
			}
		})
	}
}
