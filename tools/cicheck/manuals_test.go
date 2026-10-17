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
		"Inside an enrolled repository too, an @ reference reads such a file",
		"the hook still denies the agent's own Read of it and a shell command that names it",
		"A write is judged whole up to the read bound of 8 MiB",
		"is a notice the agent reads, never a block",
		"any other state is a denial whose reason asks for golaw.yml to be committed",
		"git status reads as clean because the index tells it not to look, or because its stat data still matches the index, is invisible to detection",
		"The name rules still judge a file over the read bound",
		"whose header the hook reads from the whole input in bounded memory",
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
		"A here-document the line never ends takes the rest of the command line as its body, as bash does, and is data unless a shell reads it",
		"A $[ ] or ${ } that holds a blank, a newline, a parenthesis or another separator of commands, where the scanner ends the word, is beyond this reading",
		"a here-document a shell reads or its line sources from the standard input",
		"a body eval runs through the output of cat, one xargs or a read loop turns into arguments and the script of another interpreter",
		"none of which a well-meaning agent writes, are read otherwise than the shell reads them; they are stated limits, not reasons to widen the reading",
		"begins a comment for this reading, while the shell keeps it in the word, so the commands after it on its line pass and a quote after it opens nothing",
		"which the shell hands the script as its positional parameters, are read as words of the script, so a comment that ends the script hides them",
		"The string of env -S is split as a POSIX shell splits words, so an escape env reads its own way is read as the character it escapes",
		"is read as bash 4 and later read it, typed at the tool's shell or fed to bash through a here-document",
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
		"Accept one JSON object of at most 8 MiB",
		"a post event it reaches is allowed with a notice",
		"A finding the hook established before the deadline is still fed back, with the deadline notice appended.",
		"Post detection never blocks on what it could not measure",
		"records the fault and proves no denial",
		"A failed run closes with the rows that fail it",
	)
}

// TestContractsCheckAnInputOverTheBound pins what both documents say the
// hook checks of an input over its bound before any size exception.
func TestContractsCheckAnInputOverTheBound(t *testing.T) {
	t.Parallel()
	requireText(t, "docs/agent-contracts.md",
		"it is checked as an event within the bound is: its JSON to the last byte and its envelope and tool input as their decoders check them",
		"NotebookEdit is denied for compatibility at any size",
		"a field it reads that holds one, and an input whose other bytes pass 8 MiB, is cannot judge",
	)
	requireText(t, "docs/security.md",
		"An input over that bound is checked as one within it, its JSON to the last byte",
		"so malformed input and an unsupported shape stay denials at any size",
	)
}

// TestContractsJudgeWhatAMoveOfHeadChanged pins what both documents say a
// command that moves HEAD has judged after it, from or to an unborn branch
// too: only what the commits the command created changed, with the files a
// move brought from other commits named in a notice, and what a move git
// cannot compare leaves.
func TestContractsJudgeWhatAMoveOfHeadChanged(t *testing.T) {
	t.Parallel()
	requireText(t, "docs/agent-contracts.md",
		"unless the command moved HEAD: then git diff-tree names, once, the paths that differ between the HEAD of the pre event and the HEAD after the command, the HEAD of an unborn branch read as the empty tree",
		"of the paths it finds changed that git status reports clean after the command, only those the commits the command created changed are judged",
		"that are no merge, were committed in the second the pre event started or later, git recording whole seconds, and carry the name and email git var GIT_COMMITTER_IDENT answers in the repository",
		"So a file a command writes and commits is judged, the first commit of its branch included",
		"every file a pull, a checkout, a merge or a reset brings from commits the command did not create is named in a notice, twenty by name and the rest counted, that asks for no fix and leaves it to the pipeline",
		"an orphan checkout, which leaves every file with the bytes the commit it left held, reports nothing",
		"Git log and git var run only when the move brought such a path",
		"A move git cannot compare, such as one from a commit the command pruned, is a notice naming the changes of the command and each path whose status alone changed",
		"reads as a commit it did not create, a stated limit",
		"a move of HEAD git cannot compare, a file git cannot convert to compare and the deadline each leave a notice the agent reads",
	)
	requireText(t, "docs/security.md",
		"of those, it judges only what the commits the command created changed, and names the files a pull, a checkout, a merge or a reset brought from other commits in a notice that asks for no fix",
		"a pre event that left no baseline, a move of HEAD git cannot compare, a file git cannot convert to compare or the deadline, is a notice the agent reads",
		"a move of HEAD git cannot compare are left to the pipeline with a notice the agent reads",
		"a file a command writes and commits is judged, the first commit of its branch included, the paths between the two HEADs being asked of git, while a file a move brings from a commit the command did not create is left to the pipeline with a notice that names it",
	)
}

// TestContractsNameWhatGitCannotConvert pins what agent-contracts.md says
// of a file whose bytes git cannot convert to compare them with the blob
// HEAD holds.
func TestContractsNameWhatGitCannotConvert(t *testing.T) {
	t.Parallel()
	requireText(t, "docs/agent-contracts.md",
		"and one git cannot convert, as when a required filter fails, is a notice naming it and never a change",
	)
}

// TestSecurityStatesThePolicyBound pins the bound init and the hooks share
// for the golaw.yml of a checkout before its first commit.
func TestSecurityStatesThePolicyBound(t *testing.T) {
	t.Parallel()
	requireText(t, "docs/security.md",
		"The golaw.yml of the checkout is read up to 1 MiB, the bound init reads it with too: init refuses to enroll a longer one",
		"one grown past it after init is that denial until git holds it, staged or committed",
	)
}

// TestSecurityStatesTheFilterDrivers pins what docs/security.md says the
// git commands of change detection run from the git configuration of the
// checkout, and why a cloned repository names nothing they run.
func TestSecurityStatesTheFilterDrivers(t *testing.T) {
	t.Parallel()
	requireText(t, "docs/security.md",
		"The git status and git hash-object of change detection run the clean filter driver the git configuration of the checkout names for a path",
		"and core.fsmonitor runs its hook, as any git command in that checkout does",
		"A cloned repository names no such program, because git configuration is not cloned",
		"the drivers are the person's own configuration and the tools they installed, such as git-lfs",
	)
}

// TestAgentContractDetection pins what docs/agent-contracts.md says change
// detection after a shell command sees and where it trusts git status.
func TestAgentContractDetection(t *testing.T) {
	t.Parallel()
	requireText(t, "docs/agent-contracts.md",
		"a submodule moved to another revision are excluded from detection",
		"a path git status reports clean at both ends is unchanged and never read",
		"A path git status names at one end only is compared by its bytes",
		"Git status trusts what the index records",
		"The name rules judge every file, one over the read bound included",
		"a write is judged by the name rules over the path it names",
		"is read to its end in bounded memory for its header",
		"leaves to the notice only the verdict that file could change",
		"a file the index marks assume-unchanged, by git update-index or core.ignorestat, or skip-worktree reads clean whatever its bytes",
		"so one git status names before and after the command is named in the notice whatever the command did",
		"asked of git hash-object once for every such path and writing no object, so a file git converts on its way in is left alone when only its status changed",
	)
}
