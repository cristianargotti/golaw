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
		"From a checkout of the kit at the validated tag",
		"wherever the session runs",
		"cover only private key and keystore files, the Terraform variable files Terraform loads by itself (terraform.tfvars and *.auto.tfvars, and their JSON form), the service account files whose name ends in service-account.json or service_account.json, SSH keys, and cloud credential files",
		"such as git -C, is not judged",
		"a write through a link into an enrolled repository is judged there",
		"the link of the file or of a directory above it, is refused as one that repository cannot place",
		"judged both by the name it asks for and by the file its links lead to",
		"keeps every element below the repository boundary, a directory link back to the root included",
		"These rules cannot express an exception",
		"a negation rule for the templates would also lift the person's own deny rules for them",
		"a template such as dev.example.tfvars, terraform.tfvars.example or service-account.example.json is readable, to Read and to @ references, as the hook allows it",
		"outside an enrolled repository nothing denies their Read",
		"A write is judged whole up to the read bound of 8 MiB",
		"is a notice the agent reads, never a block",
		"any other state is a denial whose reason asks for golaw.yml to be committed",
	)
}

// TestSecurityShellReading pins how docs/security.md says the hook reads a
// shell command line and where that reading stops.
func TestSecurityShellReading(t *testing.T) {
	t.Parallel()
	requireText(t, "docs/security.md",
		"splits its arguments by the Microsoft runtime rules",
		"past the condition of an if, the clause of a for up to its do and an else",
		"the items of a for set, the root of a for /r and the file of an if exist, stay judged",
		"the command the set of a for /f runs is read as the script of a child cmd /c",
		"call and start are read as wrappers of the command after them",
		"a redirection, wherever it stands and joined to a word or not, is no argument",
		"A command line built while it runs is beyond that reading, because reading it would need a full interpreter",
		"the input xargs reads, the lines a for /f hands its body and a variable cmd expands",
		"such as the else block of an if, are read as one command with the words before them",
		"in the body of a here-document whose end word is unquoted, never in the body of one whose end word is quoted",
		"bash 3.2, the /bin/bash of macOS, ends it at the first parenthesis left open",
		"A comment, from a hash that starts a word to the end of its line or a PowerShell block comment, is text no rule reads",
		"the lines after a here-document the line never ends are read as commands as well as its body",
		"a here-document a shell reads or its line sources from the standard input",
		"a body eval runs through the output of cat, one xargs or a read loop turns into arguments and the script of another interpreter",
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
		"every symbolic link in it is followed, the last one included, dangling or not",
		"a leading ~ or ~/ taken from the home the hook process shares with the host",
		"so a host that sends the name as the model wrote it is placed the same way",
		"then every directory that holds a link the name passes through",
		"both by the name it asks for and by the file its links lead to",
		"a submodule moved to another revision are excluded from detection",
		"a path clean against the index at both ends is unchanged and never read",
		"Accept one JSON object of at most 8 MiB",
		"a post event it reaches is allowed with a notice",
		"Post detection never blocks on what it could not measure",
		"records the fault and proves no denial",
		"A failed run closes with the rows that fail it",
	)
}
