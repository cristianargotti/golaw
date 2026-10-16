# Agent integration contracts

M1 targets Claude Code. The validated host version is 2.1.299. The minimum supported version is 2.1.295 because installed handlers require onFailure: block.

## Installation and exec form

The plugin owns hooks/hooks.json and its verbatim skill copy. Hooks do not live in settings.json. The installer adds narrow permissions.deny Read rules for private key and keystore files, the Terraform variable files Terraform loads by itself (terraform.tfvars and *.auto.tfvars, and their JSON form), the service account files whose name ends in service-account.json or service_account.json, SSH keys, and cloud credential files.

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

Accept one JSON object of at most 8 MiB, the read bound of a file, so a write the snapshot could hold is judged whole, and validate the envelope for the named adapter. An envelope from another agent is denied. A positively unenrolled location receives a valid allow. Missing or weakened policy in an enrolled repository is denied. An input over the bound is answered from the header its first 8 MiB hold, the host writing the tool input after the rest of the header: a post event is allowed with a notice naming what was not measured, a write is judged by the name rules over the path it names, so a name the tree forbids is denied and any other is allowed with that notice, a write of golaw.yml stays denied, and any other event, or a write whose path comes after its content, is cannot judge.

The location of a write or read is the deepest existing directory above tool_input.file_path, or notebook_path for NotebookEdit, after the name is expanded the way the host expands it, trimmed, a leading ~ or ~/ taken from the home the hook process shares with the host, on Windows a /c/ name read as that drive, and cleaned, and every symbolic link in it is followed, the last one included, dangling or not. Claude Code 2.1.299 already sends the hook the path it expanded; the hook expands it again, so a host that sends the name as the model wrote it is placed the same way. A name that starts from the home is cannot judge when the hook process has no home, and the Unicode normal form the host also applies is not mirrored. When no enrolled repository holds that file, the directory that holds the entry the name opens is tried next, then every directory that holds a link the name passes through, so a link, of the file or of a directory above it, carries a write or a read neither into nor out of an enrolled repository unjudged; a write it carries out of one is refused as cannot judge. The repository that holds the target governs it whatever cwd the session reports. A read outside every enrolled repository falls back to the repository of cwd. A shell event is located at cwd.

Change detection after a shell command asks git status, with the ignore rules of the universe and without taking the index lock, for the paths that differ from HEAD in the index, from the index in the worktree, or that git would add, before the command and after it, and identifies only those: a path git status reports clean at both ends is unchanged and never read, so a file a command both writes and commits is left to the pipeline. Git status trusts what the index records: a file the index marks assume-unchanged, by git update-index or core.ignorestat, or skip-worktree reads clean whatever its bytes, and so does a file rewritten to its size with its modification time put back, as cp -p, tar or touch -r leave it, while its stat data still matches the index, which core.checkStat minimal with core.trustctime false always lets it; a command that rewrites such a file is invisible to detection at both ends, without a notice, and left to the pipeline. A path git status names at one end only is compared by its bytes: one named before the command only is identified as it is after it, and one named after it only is unchanged while its bytes are still the blob the HEAD of the pre event held there, so a mode change, a commit of bytes already dirty and a soft reset that leaves a committed file staged report nothing, while a file written back to its committed content is judged; the bytes are compared as they lie on disk, so a file git converts on its way in, by an end of line rule or a filter, is judged when only its status changed. It identifies a symbolic link by its class and the length of its link text and a submodule by its class alone, because the snapshot holds the bytes of neither, the model contract carries neither a link text nor a submodule revision, and no check follows a link or enters a submodule: a link already changed against the index and retargeted again to a text of the same length and a submodule moved to another revision are excluded from detection. Neither carries a content rule, so the exclusion hides no finding.

## Exit and output contract

| Situation | Exit | Output |
| --- | --- | --- |
| Allowed pre event | 0 | Valid allow result |
| Judged pre denial | 0 | hookSpecificOutput.permissionDecision is deny; permissionDecisionReason is English; systemMessage uses the configured human language |
| Cannot judge | 2 | Nonempty English stderr reason |
| Post detection | 0 | decision is block with a reason; execution has already happened |
| Post failure detection | 0 | additionalContext describes the detected change |
| Allowed with a gap | 0 | hookSpecificOutput.additionalContext of the event names what was not measured and that the pipeline judges it; systemMessage names the same path in the configured language |
| Unexpected process failure | Not emitted intentionally | Host applies onFailure: block |
| Host timeout or missing binary | No binary answer | Installed onFailure: block must stop the tool, established live |

The internal deadline is shorter than the ten second host timeout. Reaching it before a pre event is answered produces a denial; a post event it reaches is allowed with a notice, because its tool already ran. Post feedback never claims rollback.

Post detection never blocks on what it could not measure: a file over the read bound, identified by its size so a command that leaves it alone leaves it unnamed, a change set over 65536 paths or 64 MiB, a pre event that left no baseline and the deadline each leave a notice the agent reads, naming the path when there is one, and the pipeline judges what they left; a file it could measure is still judged, and a finding still blocks. The name rules judge every file, one over the read bound included, so a name the tree forbids is fed back after a command and denied before a write although its bytes are not measured; only a rule that needs the bytes the hook does not hold leaves the file to the notice. A Write over a file beyond the read bound is judged on its whole content, and an Edit of such a file, which cannot be reconstructed within the bound, is judged by the name rules and otherwise allowed with the same notice.

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
