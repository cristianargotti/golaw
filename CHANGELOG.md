# Changelog

Every rule change is dated here with its reason. The format follows Keep a Changelog and the versions follow semantic versioning.

## Unreleased

### Added

- 2026-10-14: the repository scaffold: the module, the laws, the local pipeline, the strict lint configuration and the decision records for cobra and yaml.
- 2026-10-14: the command tree (version, scan, init, config explain, coexist), the policy of the kit itself, the coverage, publication age and vulnerability tools, the three operating system CI and the scheduled vulnerability lane.
- 2026-10-14: the command tree over the integrated packages: `hook <agent>`, `agent install` and `agent uninstall`, `doctor`; the kit scans itself at zero and `make ci` is green on macOS.
- 2026-10-15: the text family, rule rules/4.1@1: text.dashes, text.trace-terms, text.tool-terms, text.comment-language, text.todo, text.bare-nolint, text.nolint-nilnil and text.debug-print, because the laws of section 3 hold for every text file an agent writes and scan reads.
- 2026-10-15: the paths family, rule rules/4.2@1: paths.forbidden, paths.generated-tracked and paths.secret-material, because a forbidden file, a hidden generated tree or a read of secret material is judged by its name before any content.
- 2026-10-15: the goast family, rule rules/4.3@1: the seventeen Go source checks docs/rules.md lists, from goast.file-length to goast.sonar-identical-functions, because the size, clock, error and secret rules need the parsed syntax tree.
- 2026-10-15: the shell family: the commit laws a hook sees, rule rules/4.12@1 (shell.no-verify, shell.trailers, shell.dashes, shell.body-width, shell.shape), and the hook denials, rule rules/4.7@1 (shell.secret-tokens, shell.credential-print, shell.push, shell.pr), because a command is judged before it runs and never executed.
- 2026-10-16: the acceptance report marks every row required or optional and closes a failed run with the rows that fail it, because a release run stopped by a skipped live row did not say which row stopped it.

### Changed

- 2026-10-16: the hook resolves enrollment from the target of a write or read, and a read outside every enrolled repository falls back to the session's repository, because a write or read into an enrolled repository from an unenrolled working directory was allowed and a write to the agent's own plans from an enrolled one was refused as cannot judge.
- 2026-10-16: agent install re-enrolls a committed golaw.yml that tightens the enrolled policy and refuses one that weakens it, and init --apply keeps an existing golaw.yml, because a reviewed change left the record behind with no way back and the advised init --apply rendered the defaults over the reviewed file.
- 2026-10-16: the text family and paths.generated-tracked answer unmeasured for a file the snapshot could not read, and a tracked file deleted from the worktree leaves the snapshot, because an unreadable file scanned clean and a deletion would then have read as unreadable.
- 2026-10-16: agent install writes Read deny rules for private keys, Terraform variable files, the SSH keys and the cloud credential stores only, because the user settings apply them to every project and the full name list denied files the hook allows.
- 2026-10-16: paths.secret-material and shell.secret-tokens drop the singular secret directory and the bare secrets file names, spare the fixtures under a secrets package, read a template marker before the extension, skip a Go package pattern and judge a read by its path from the repository root, because they denied ordinary commands and files section 8.4 does not list.
- 2026-10-16: the shell family reads a command in the dialect of its shell, with the backslash as a path separator under PowerShell and cmd, because native Windows paths escaped the git and secret rules.
- 2026-10-16: text.trace-terms no longer refuses the bare pipeline word and counts the trailer phrases in trailer form only, because it denied ordinary prose of adopting repositories.
- 2026-10-16: text.tool-terms reads an allowed path without a final slash as one file, because a file scope also allowed every path it was a prefix of.
- 2026-10-16: text.comment-language no longer counts el and la, because they are Go names an English comment talks about.
- 2026-10-16: doctor names the validated host beside the floor and reports a newer one as unknown, judges the pin by the release rule and ends with the last dispatch evidence, because a newer host read as supported, a development build read red and no line reported dispatch.
- 2026-10-16: the coexistence check reads the gate in the configuration directory of the agent, because with its override set agent install wrote one settings file and the check read another.
- 2026-10-16: the skill names the rules page of the kit at its release tag, because M1 init writes no docs/rules.md in an adopting repository.
- 2026-10-16: the empty output acceptance row is recorded as a protocol limit, because exit 0 with no output is the allow of the host and proves no denial.
