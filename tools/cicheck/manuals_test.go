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
		"So is a finding of a check the deadline stopped between two of its files, which the check established whole before the deadline.",
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
		"that were committed in the second the pre event started or later, git recording whole seconds, and carry the name and email git var GIT_COMMITTER_IDENT answers in the repository",
		"Of a merge among them, the files it holds apart both from every parent, as git log --cc names them in the same question, and from the merge git makes of its parents by itself, as git log --remerge-diff names them, once for every such merge, are judged",
		"while a file it took from one side whole and one git merged by itself are named in the notice",
		"a git older than 2.36, which has no --remerge-diff, or a remerge git fails leaves each file such a merge holds apart from every parent to a notice naming it",
		"and the resolution a merge commit of the command holds",
		"and git log --remerge-diff only when a merge the command made holds one apart from every parent",
		"So a file a command writes and commits is judged, the first commit of its branch included",
		"every file a pull, a checkout, a merge or a reset brings from commits the command did not create is named in a notice, twenty by name and the rest counted, that asks for no fix and leaves it to the pipeline",
		"an orphan checkout, which leaves every file with the bytes the commit it left held, reports nothing",
		"A commit the command created on the parents of the HEAD the pre event started from by a git commit --amend its text runs plainly, an amend, rewrites that HEAD: every file the rewrite changed against it is judged",
		"a file the amended commit drops from its delta and restores to the bytes of its parent included",
		"a reset followed by a commit on the same parents is no amend, so the notice names what the reset brought, and an amend a script runs is not read as one, a stated limit",
		"Git log and git var run only when the move brought such a path",
		"A move git cannot compare, such as one from a commit the command pruned, is a notice naming the changes of the command and each path whose status alone changed",
		"reads as a commit it did not create, a stated limit",
		"a move of HEAD git cannot compare, a file git cannot convert to compare and the deadline each leave a notice the agent reads",
		"the paths of one cause in one sentence that names twenty and counts the rest, for the agent and in the language of the policy for the person, so a pull git cannot classify, when git var cannot name the committer, is one bounded sentence",
	)
	requireText(t, "docs/security.md",
		"of those, it judges only what the commits the command created changed, and names the files a pull, a checkout, a merge or a reset brought from other commits in a notice that asks for no fix",
		"a pre event that left no baseline, a move of HEAD git cannot compare, a file git cannot convert to compare or the deadline, is a notice the agent reads",
		"a move of HEAD git cannot compare are left to the pipeline with a notice the agent reads",
		"a file a command writes and commits is judged, the first commit of its branch included, the paths between the two HEADs being asked of git, while a file a move brings from a commit the command did not create is left to the pipeline with a notice that names it",
		"of a merge commit the command made, it judges what the merge holds apart both from every parent and from the merge git makes of its parents by itself",
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

// TestContractsJudgeTheHostDiffAsDetectionDoes pins what agent-contracts.md
// says the post event of a shell command does with the files the host names
// in bashEditDiff.
func TestContractsJudgeTheHostDiffAsDetectionDoes(t *testing.T) {
	t.Parallel()
	requireText(t, "docs/agent-contracts.md",
		"A file bashEditDiff names gets the answer that detection gives it",
		"the files bashEditDiff names are judged as they stand only when no baseline classifies what the command changed",
	)
}

// TestContractsLeaveWhatAMergeBrought pins what both documents say of the
// files git wrote for a merge the command started, judged against the merge
// git makes by itself, and of a conflict git left unmerged.
func TestContractsLeaveWhatAMergeBrought(t *testing.T) {
	t.Parallel()
	requireText(t, "docs/agent-contracts.md",
		"so a command that leaves HEAD in place asks git nothing more, unless it starts a merge it leaves in progress",
		"A path git status reports unmerged after the command that it did not name before holds the conflict git wrote from both sides",
		"it is named in a notice and judged by the event that writes its resolution",
		"When the command started the merge, its bytes are first compared with that conflict, the merge git makes by itself with the strategy options of the command",
		"bytes HEAD holds whole are unchanged and report nothing",
		"any other bytes, a resolution the command wrote without git add, are judged in the same event",
		"a resolution the command writes for a conflict a rebase, a cherry-pick, a revert or a stash left, which no MERGE_HEAD names, waits for a later event, a stated limit",
		"A command that starts a merge it leaves in progress, MERGE_HEAD present after it and not before, read as a file",
		"has git merge-tree --write-tree make, once, the merge of HEAD and the commit MERGE_HEAD names as git makes it by itself",
		"so the repository receives no object",
		"a file whose staged blob is one of those two is named in a notice that asks for no fix, a file git merged from both sides and a conflict resolved with the side of that commit taken whole included",
		"a file whose staged blob is neither, the command's resolution of a conflict or its own edit, a file only the local side added or changed since the fork included, is judged",
		"a merge already in progress before the command leaves the files the command writes judged as any other",
		"A file a commit the command created before it started that merge changed, as when it commits its own change and then merges, is judged whatever the merge staged there",
		"as is such a file the command left unmerged with the bytes HEAD holds",
		"A merge git cannot make by itself to compare, an octopus merge, one under a git older than 2.38, which has no merge-tree --write-tree, or one git fails, leaves each file the merge staged to a notice naming it",
		"A merge made with the strategy option -X ours or -X theirs, left in progress or committed, is also compared with the merge git makes with that option",
		"which the pre event reads from a git merge or git pull the text of the command runs plainly, in each spelling git takes",
		"Another strategy option, an -X of another program or after the git command ends, and an option an alias, a script or the configuration gives, is not read, so a file such an option resolved hunk by hunk is judged as the command's resolution, a stated limit",
	)
	requireText(t, "docs/security.md",
		"A merge the command starts and leaves in progress is read the same way: a file it staged whose bytes are those of the merge git makes by itself or of the commit it merges is named in that notice",
		"a file whose staged bytes are neither, the command's own resolution or edit, is judged",
		"a file git leaves unmerged waits for the event that writes its resolution",
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
		"The git merge-tree that compares a merge the command left in progress, and the git log --remerge-diff that compares a merge commit the command made, with the merge git makes by itself run the merge driver the git configuration of the checkout names for a path",
		"keep the objects of that merge in a temporary object directory removed after, so none reaches the repository",
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
		"identified by the size, modification time, change time and inode of its lstat",
		"on Windows its size and modification time only",
		"a command that leaves it alone leaves no notice",
		"a rewrite that changes any of them is named",
		"a rewrite that restores them all is the stat limit of git status stated above",
		"asked of git hash-object once for every such path and writing no object, so a file git converts on its way in is left alone when only its status changed",
	)
}
