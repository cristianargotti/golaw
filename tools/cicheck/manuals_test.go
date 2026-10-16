package cicheck_test

import "testing"

func TestSecurityContract(t *testing.T) {
	t.Parallel()
	requireText(t, "docs/security.md",
		"Linux", "macOS", "Windows",
		"not a sandbox",
		"only hard gate",
		"staged scanning",
		"execute no repository code",
		"symlinked",
		"Inherited environment variables",
		"long-lived credentials",
		"last validated kit tag",
		"kit.version",
		"enrollment digests",
		"never lowering a policy floor",
		"time-boxed",
		"no repository-level advisory switch",
		"recovery-control",
		"wherever the session runs",
		"cover only private key files, Terraform variable files, SSH keys, and cloud credential files",
		"such as git -C, is not judged",
	)
}

func TestAgentContract(t *testing.T) {
	t.Parallel()
	requireText(t, "docs/agent-contracts.md",
		"2.1.299", "2.1.295",
		agentHandlerJSON(t),
		"PostToolUseFailure",
		"additionalContext",
		"| Cannot judge | 2 |",
		"Offline acceptance",
		"Only a live record",
		"2026-10-14",
		"sleep 20",
		"hello.txt remained absent",
		"predecessor guard",
		"standalone project settings hook",
		"two turns at 0.53 USD",
		"Neither identifies a golaw binary",
		"does not prove the missing-binary case",
		"SKIP never counts as PASS",
		agentID(t)+" plugin validate --strict is a required row",
		"docs/live/"+agentID(t)+"-darwin.json",
		"-buildvcs=false",
		"Go "+gateToolchain(t)+", the go-version of release.yml",
		"waits for the Windows live record",
		"doctor-json",
		"deepest existing directory above tool_input.file_path",
		"records the fault and proves no denial",
		"A failed run closes with the rows that fail it",
	)
}
