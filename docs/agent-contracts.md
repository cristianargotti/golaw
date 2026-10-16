# Agent integration contracts

M1 targets Claude Code. The validated host version is 2.1.299. The minimum supported version is 2.1.295 because installed handlers require onFailure: block.

## Installation and exec form

The plugin owns hooks/hooks.json and its verbatim skill copy. Hooks do not live in settings.json. The installer adds narrow permissions.deny Read rules for private keys, Terraform variable files, SSH keys, and cloud credentials.

Each handler uses exec form:

```json
{
  "type": "command",
  "command": "golaw",
  "args": [
    "hook",
    "claude"
  ],
  "timeout": 10,
  "onFailure": "block"
}
```

No shell parses that invocation. doctor checks that executable resolution identifies the running binary. Windows acceptance proves that the bare name resolves to golaw.exe through PATH. The fallback of the installer copying the binary under ${CLAUDE_PLUGIN_DATA} is not built in M1: it waits for the Windows live record and ships only if the bare name fails there.

## Events

| Event | Matcher | Contract |
| --- | --- | --- |
| PreToolUse | Write, Edit, NotebookEdit | Write supplies file_path and content. Edit supplies old_string, new_string, replace_all, and edits when present. Reconstruct the candidate in memory. Deny NotebookEdit with a compatibility reason until decoded. |
| PreToolUse | Read | Judge tool_input.file_path against secret-material rules, both by the name it asks for and by the file its links lead to. |
| PreToolUse | Bash, PowerShell | Judge command text, cwd, and tool name without executing the command. |
| PostToolUse | Write, Edit, NotebookEdit | Measure resulting content when prospective measurement was unavailable. |
| PostToolUse | Bash, PowerShell | Detect changed content from bashEditDiff when present or the tool_use_id snapshot. |
| PostToolUseFailure | Bash, PowerShell | Detect changes even when the command failed after writing. |

Accept one bounded JSON object and validate the envelope for the named adapter. An envelope from another agent is denied. A positively unenrolled location receives a valid allow. Missing or weakened policy in an enrolled repository is denied.

The location of a write or read is the deepest existing directory above tool_input.file_path, or notebook_path for NotebookEdit, after the name is trimmed and cleaned the way the host expands it and every symbolic link in it is followed, the last one included, dangling or not. When no enrolled repository holds that file, the directory of the link the name opens is tried next, so a link carries a write or a read neither into nor out of an enrolled repository unjudged. The repository that holds the target governs it whatever cwd the session reports. A read outside every enrolled repository falls back to the repository of cwd. A shell event is located at cwd.

## Exit and output contract

| Situation | Exit | Output |
| --- | --- | --- |
| Allowed pre event | 0 | Valid allow result |
| Judged pre denial | 0 | hookSpecificOutput.permissionDecision is deny; permissionDecisionReason is English; systemMessage uses the configured human language |
| Cannot judge | 2 | Nonempty English stderr reason |
| Post detection | 0 | decision is block with a reason; execution has already happened |
| Post failure detection | 0 | additionalContext describes the detected change |
| Unexpected process failure | Not emitted intentionally | Host applies onFailure: block |
| Host timeout or missing binary | No binary answer | Installed onFailure: block must stop the tool, established live |

The internal deadline is shorter than the ten second host timeout. Reaching it produces a denial. Post feedback never claims rollback.

## Trust

Interactive settings-file hooks wait for folder trust. Noninteractive sessions can run those hooks without that dialog. Those facts do not establish plugin behavior before workspace trust. doctor reports plugin trust as unknown until live evidence establishes it.

Keeping hooks in the plugin avoids the settings-file hook import used by Cursor. Installation, enablement, integrity, trust, and observed dispatch remain separate facts.

## Offline acceptance

The M1 matrix runs on Linux, macOS, and Windows. It installs twice, checks unchanged receipts and foreign settings, and invokes the exact installed command with closed stdin in an isolated home without credentials.

Required rows deny a dash write, a secret read, and a --no-verify commit; allow their controls; and deny malformed input, a crash, and the forced internal slow path.

The empty output row, in the protocol-limit category, records the fault and proves no denial: the host treats exit 0 with no output as an allow, which is also the valid allow the adapter answers, so a binary that exits cleanly without an answer is a limit of the protocol that no hook closes.

The doctor-json row decodes doctor --json after an installed hook ran: every artifact matches its receipt, the hook names the built binary, every hook blocks on failure, and a source build is reported as metadata, never verified.

Each result identifies category, agent, OS, agent version, binary digest, PASS, FAIL, or SKIP, and whether the milestone requires the row. SKIP never counts as PASS. A failed run closes with the rows that fail it, so a release run stopped by a skipped required row names that row.

claude plugin validate --strict is a required row. claude plugin test is the host's runner for hook modules written against its testing kit; the plugin ships command hooks and no module, so the host refuses the command and the row is recorded as not applicable, never as a pass. Neither proves installation or live dispatch. The kit also validates handler fields because the vendor validator accepted an unknown field during the initial investigation.

Offline acceptance proves the adapter and installed invocation. Only a live record proves dispatch, trust behavior, host failure behavior, and resulting target state.

## Live record: 2026-10-14

Host: Claude Code 2.1.299 on macOS. The daily denials of 2026-10-14 came from the predecessor guard during normal use.

The timeout probe was a standalone project settings hook, separate from that guard. Its PreToolUse Write handler ran sleep 20 with timeout 3 and onFailure: block. It was executed through claude -p in two turns at 0.53 USD. The host refused the write, hello.txt remained absent, and the reported reason was: timed out; blocking because onFailure is "block".

Both records are host evidence for Claude Code 2.1.299 on macOS. Neither identifies a golaw binary or its digest, establishes dispatch of the golaw plugin, or authenticates a release. The timeout probe does not prove the missing-binary case.

Before M1 release, the owner records one dated golaw session on macOS in docs/live/claude-darwin.json: an allowed control, a denied write, a host timeout and a missing binary, each with its outcome and target state. The denied write, the timeout and the missing binary must leave the target unchanged. The timeout case puts a program named golaw that sleeps past the ten second timeout first on PATH; the missing binary case takes golaw off PATH.

That record names the owner, environment, credential boundary, spending limit, schedule, retention, agent version, and the SHA-256 of the binary the session installed: the reproducible darwin/arm64 build of the tagged source, CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -buildvcs=false ./cmd/golaw with Go 1.27.0, the go-version of release.yml. A commit that only adds the record leaves that build unchanged; a toolchain bump changes it and needs a new record.

go run ./tools/acceptance reads the record: the host-timeout and host-missing-binary rows pass only when the record is complete and names the candidate digest. An absent record, or one naming another binary, is SKIP. The release lane runs the harness with -release before any draft, and SKIP never counts as PASS there, so no tag publishes without the record.

Linux and Windows have no live M1 enforcement claim. Codex CLI and Cursor are outside M1.

## Compatibility history

| Entry | Version | Evidence |
| --- | --- | --- |
| M1 host contract | Claude Code 2.1.299 | Historical macOS probe dated 2026-10-14; released-candidate evidence remains a release requirement |
| Minimum host | Claude Code 2.1.295 | Required for onFailure: block |

Retain the previous validated kit and agent combination when adding a release. Follow security.md for recovery.
