# Security model

golaw checks repository content and supported agent operations. It is not a sandbox. An agent holding credentials can still act outside the guarded routes.

## Assurance levels

| Level              | Evidence                                                                       | Claim                                                |
| ------------------ | ------------------------------------------------------------------------------ | ---------------------------------------------------- |
| Rule proof         | Production dispatch, red and near-miss fixtures, deletion and over-fire passes | The check distinguishes its intended violation       |
| Offline acceptance | The exact installed invocation receives recorded inputs                        | The adapter and launcher answer the protocol         |
| Live acceptance    | A dated record identifies agent, OS, version, binary, and target state         | The host dispatched and respected the installed hook |

| Agent       | OS                    | M1 offline scope                                          | M1 live scope                       |
| ----------- | --------------------- | --------------------------------------------------------- | ----------------------------------- |
| Claude Code | Linux                 | Adapter and installed exec invocation                     | No live enforcement claim           |
| Claude Code | macOS                 | Adapter and installed exec invocation                     | Required for the released candidate |
| Claude Code | Windows               | Adapter, installed exec invocation, executable resolution | No live enforcement claim           |
| Codex CLI   | Linux, macOS, Windows | Outside M1                                                | Outside M1                          |
| Cursor      | Linux, macOS, Windows | Outside M1                                                | Outside M1                          |

These are release requirements, not a substitute for the recorded acceptance results. The historical macOS host evidence is in agent-contracts.md.

## What the guard stops

In an enrolled repository, supported pre events deny a typographic dash write, a prohibited secret read, a --no-verify commit, and an oversized candidate whose whole content can be reconstructed. An Edit is applied in memory before measurement. A shell command is read the way its shell hands the program its words: a redirection, wherever it stands and joined to a word or not, is no argument, and the name rules judge its file. A comment, from a hash that starts a word to the end of its line or a PowerShell block comment, is text no rule reads, so a quote or a here-document operator in it opens nothing.

The --no-verify commit is recognized where the command line spells it: behind assignments, the reserved words of a shell and the wrappers it names, each wrapper past its own options, in the script a shell, eval, Invoke-Expression, a here-document a shell reads, a substitution or the set of a cmd for /f runs, and in the command xargs, call and start run. A substitution is read where the shell expands it: inside a word, outside single quotes, and in the body of a here-document whose end word is unquoted, never in the body of one whose end word is quoted. A command line built while it runs is beyond that reading, because reading it would need a full interpreter: the value of a variable or the output of a command the line expands, the input xargs reads, the lines a for /f hands its body and a variable cmd expands. The blocks of a PowerShell statement after its first, such as the else block of an if, are read as one command with the words before them. A substitution inside double quotes ends where bash 4 and later, zsh and dash end it, at the parenthesis that matches it past the quotes, the here-document bodies and the case patterns inside it; bash 3.2, the /bin/bash of macOS, ends it at the first parenthesis left open, so a line written to read otherwise there is beyond this reading.

The repository that holds the file a write or read names governs it, wherever the session runs. A symbolic link changes neither: the file a name leads to through every link is placed first, then the link the name opens and every directory that holds a link it passes through, so a write through a link into an enrolled repository is judged there, and a write through a link of an enrolled repository to a file outside it, the link of the file or of a directory above it, is refused as one that repository cannot place. A write outside every enrolled repository, such as the agent's own plan files, is a positively unenrolled location. A read outside every enrolled repository is judged by the session's repository, so the person's credential stores stay behind the secret boundary of an enrolled session.

Malformed input, an unreadable target, an envelope mismatch, and an unsupported mutation shape are denials. The internal deadline is shorter than the host timeout. The installed handler uses timeout 10 and onFailure: block, requiring Claude Code 2.1.295 or newer.

Post events detect changes after execution. They cannot undo writes. Shell post failure detection also runs because a command can write before a later operation fails.

## What the guard cannot stop

Background writers, manual edits, hosted tools, and MCP tools outside supported event routes can bypass the guard. Claude Code @ references bypass hooks; narrow Read deny rules cover only their listed paths.

A shell command is judged by the repository of the session's working directory, because its targets cannot be known before it runs. A command sent from an unenrolled directory into an enrolled repository, such as git -C, is not judged. A script run through cmd /c is read the way cmd separates its commands, past the condition of an if, the clause of a for up to its do and an else, whose names, the items of a for set, the root of a for /r and the file of an if exist, stay judged, the command the set of a for /f runs is read as the script of a child cmd /c, call and start are read as wrappers of the command after them, and the started program splits its arguments by the Microsoft runtime rules; a program that parses its command line another way, a cmd builtin included, may read the same text differently.

Codex CLI and Cursor enforcement are outside M1. Their later adapters have separate limits: Codex write_stdin into an existing interpreter, untrusted or failed hooks, and specialized tool routes; Cursor writes until its mutation payload is understood. M1 implies no enforcement by those adapters.

## Local aids and the remote gate

Every local check is optional. Installing an agent guard opts into its denials; agent uninstall removes that integration. Local git checks are armed only by the developer and may be disabled without a finding.

M1 scans the worktree. M2 adds staged scanning and the optional git rungs. Both scans are local aids. Remote CI or the platform pipeline is the only hard gate.

An adopting platform must demonstrate its remote stage before claiming that the kit is enforced there. Without that stage, the kit runs only where developers choose to run or install it.

## Untrusted repositories

Agent hooks execute no repository code. They use trusted git plumbing, keep snapshots in memory, and never execute intercepted command text, make targets, generators, or repository tools.

init writes only within the real repository root and refuses symlinked path components. It never arms git hooks. Templates contain no project-level agent files.

Policy authority is committed HEAD, or the enrolled bootstrap policy on an unborn branch. The indexed policy is a proposed replacement and cannot weaken the policy in force. Agent edits to golaw.yml are denied.

Missing or mismatched policy in an enrolled repository is an error. A positively unenrolled location receives a valid allow.

A path exception by hash covers only an installed agent artifact recorded by the installer. It never exempts source files.

## Secret boundary

The first layer denies recognized secret-material names in Read and shell inputs. A Read is judged both by the name it asks for and by the file its links lead to, so a link named like secret material and a link to it are both denied. The name it asks for keeps every element below the repository boundary, a directory link back to the root included. The second denies known credential-printing commands. Their exact patterns are executable data with red and near-miss fixtures.

The Read deny rules the installer writes for the @ reference gap cover only private key files, Terraform variable files, SSH keys, and cloud credential files. They live in the user settings, which the host applies to every project before any hook runs, so the broad name list stays with the hook, which judges each read in the repository that governs it. These rules cannot express an exception of their own: the host reads a bracket class without negation, and it joins every deny rule of the user settings in one matcher, so a negation rule for the templates would also lift the person's own deny rules for them. A template named like a file of those families, such as dev.example.tfvars or service-account.example.json, is therefore denied to Read and to @ references even though the hook allows it; a shell command that names it is still judged by the hook, whose name list allows a template.

Inherited environment variables and secrets outside the recognized names remain exposed. Run the agent under an identity without long-lived credentials. Name checks do not prove that arbitrary file content is free of secrets.

## Recovery

Run golaw doctor before upgrading the kit or agent. Compare installed and validated versions, artifact hashes, resolved executable, policy pin, enrollment digests, and fail mode.

When a payload change, panic, or missing executable blocks work, a person selects the last validated kit tag from the compatibility history. Install that tag with go install or its signed release asset, then run golaw agent install --agent claude.

Restore the matching kit.version and enrollment digests as part of the person's reviewed change, committing the repository policy change and updating external enrollment through agent install, which enrolls the committed policy only as a tightening, never lowering a policy floor. The binary and pin must move together.

Run doctor again. From a checkout of the kit at the validated tag, since the harness ships with the kit and never with an adopting repository, replay the recovery-control row of go run ./tools/acceptance, which takes the binary away, puts the validated one back, runs agent install again and requires the allowed write through the installed invocation, before resuming work.

Retain the previous validated combination in the compatibility history after every upgrade. The first release has no previous golaw release; its acceptance demonstrates reinstalling the validated candidate.

Any emergency relaxation belongs outside the repository in user scope. It must be time-boxed, owned, audited, and prominent in doctor. There is no repository-level advisory switch.
