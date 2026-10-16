# Security model

golaw checks repository content and supported agent operations. It is not a sandbox. An agent holding credentials can still act outside the guarded routes.

## Assurance levels

| Level | Evidence | Claim |
| --- | --- | --- |
| Rule proof | Production dispatch, red and near-miss fixtures, deletion and over-fire passes | The check distinguishes its intended violation |
| Offline acceptance | The exact installed invocation receives recorded inputs | The adapter and launcher answer the protocol |
| Live acceptance | A dated record identifies agent, OS, version, binary, and target state | The host dispatched and respected the installed hook |

| Agent | OS | M1 offline scope | M1 live scope |
| --- | --- | --- | --- |
| Claude Code | Linux | Adapter and installed exec invocation | No live enforcement claim |
| Claude Code | macOS | Adapter and installed exec invocation | Required for the released candidate |
| Claude Code | Windows | Adapter, installed exec invocation, executable resolution | No live enforcement claim |
| Codex CLI | Linux, macOS, Windows | Outside M1 | Outside M1 |
| Cursor | Linux, macOS, Windows | Outside M1 | Outside M1 |

These are release requirements, not a substitute for the recorded acceptance results. The historical macOS host evidence is in agent-contracts.md.

## What the guard stops

In an enrolled repository, supported pre events deny a typographic dash write, a prohibited secret read, a --no-verify commit, and an oversized candidate whose whole content can be reconstructed. An Edit is applied in memory before measurement.

The repository that holds the file a write or read names governs it, wherever the session runs. A write outside every enrolled repository, such as the agent's own plan files, is a positively unenrolled location. A read outside every enrolled repository is judged by the session's repository, so the person's credential stores stay behind the secret boundary of an enrolled session.

Malformed input, an unreadable target, an envelope mismatch, and an unsupported mutation shape are denials. The internal deadline is shorter than the host timeout. The installed handler uses timeout 10 and onFailure: block, requiring Claude Code 2.1.295 or newer.

Post events detect changes after execution. They cannot undo writes. Shell post failure detection also runs because a command can write before a later operation fails.

## What the guard cannot stop

Background writers, manual edits, hosted tools, and MCP tools outside supported event routes can bypass the guard. Claude Code @ references bypass hooks; narrow Read deny rules cover only their listed paths.

A shell command is judged by the repository of the session's working directory, because its targets cannot be known before it runs. A command sent from an unenrolled directory into an enrolled repository, such as git -C, is not judged.

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

The first layer denies recognized secret-material names in Read and shell inputs. The second denies known credential-printing commands. Their exact patterns are executable data with red and near-miss fixtures.

The Read deny rules the installer writes for the @ reference gap cover only private key files, Terraform variable files, SSH keys, and cloud credential files. They live in the user settings, which the host applies to every project before any hook runs, so the broad name list stays with the hook, which judges each read in the repository that governs it.

Inherited environment variables and secrets outside the recognized names remain exposed. Run the agent under an identity without long-lived credentials. Name checks do not prove that arbitrary file content is free of secrets.

## Recovery

Run golaw doctor before upgrading the kit or agent. Compare installed and validated versions, artifact hashes, resolved executable, policy pin, enrollment digests, and fail mode.

When a payload change, panic, or missing executable blocks work, a person selects the last validated kit tag from the compatibility history. Install that tag with go install or its signed release asset, then run golaw agent install --agent claude.

Restore the matching kit.version and enrollment digests as part of the person's reviewed change, committing the repository policy change and updating external enrollment through agent install, which enrolls the committed policy only as a tightening, never lowering a policy floor. The binary and pin must move together.

Run doctor again. From a checkout of the kit at the validated tag, since the harness ships with the kit and never with an adopting repository, replay the recovery-control row of go run ./tools/acceptance, which takes the binary away, puts the validated one back, runs agent install again and requires the allowed write through the installed invocation, before resuming work.

Retain the previous validated combination in the compatibility history after every upgrade. The first release has no previous golaw release; its acceptance demonstrates reinstalling the validated candidate.

Any emergency relaxation belongs outside the repository in user scope. It must be time-boxed, owned, audited, and prominent in doctor. There is no repository-level advisory switch.
