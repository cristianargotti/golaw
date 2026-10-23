# Setup

M2 provides worktree and staged scanning, core templates, optional git rungs, and the Claude Code guard. M3 adds the capabilities and the policy-file checks, described below and in [rules.md](rules.md). M4 adds the Codex CLI guard and the Cursor compatibility preview, installed as the [agent section](#codex-and-cursor-agents) below describes.

Adopting repositories need Go 1.24 or newer. Building the kit needs Go 1.27. Claude Code 2.1.295 is the minimum host version; 2.1.299 is validated.

## Install and enroll

golaw setup is optional. Include that step only when you want local git rungs; agent install separately opts into the agent guard.

The adoption commit stages every file listed in the reviewed init diff. That includes golaw.yml and the generated templates. The adoption commit and every init --update commit use the feat type, because commit.shape holds a commit that is not feat to 40 new files and 400 new lines. The rendered templates exceed those limits.

The sequence below shows the default core paths. Adjust the staging list to the actual diff, including an existing decisions directory selected by init, and omit paths that init did not change. Exclude unrelated edits from the adoption commit; when a file also holds unrelated edits, stage only the reviewed adoption hunks.

init distinguishes writes from proposals for existing files. init --apply leaves proposed bytes untouched. Review and apply the accepted proposal hunks yourself before staging, including the include of golaw.mk in an existing Makefile. Unapplied proposals can be displayed again on a repeated init.

go.mod stays your file: init proposes the require of github.com/quasilyte/go-ruleguard/dsl, the module scripts/lint/rules.go imports, and prints the go get and go mod tidy that apply it with its go.sum lines. Until go.mod requires it, golangci-lint cannot load the rules file, make lint fails, and an armed pre-commit rung names that repair and leaves the lint to the pipeline.

```sh
go install github.com/cristianargotti/golaw/cmd/golaw@v0.2.0
golaw init
golaw init --apply
go get github.com/quasilyte/go-ruleguard/dsl@v0.3.23
go mod tidy
git add golaw.yml .editorconfig .gitattributes .github/dependabot.yml .github/workflows/ci.yml .gitleaks.toml .golangci.yml .gremlins.yaml .mockery.yaml Makefile golaw.mk lefthook.yml scripts/lint/rules.go test/deadcode/reachability_test.go tools/go.mod go.mod go.sum docs/adr/0001-inherited-baseline.md
git commit -m "feat: adopt the golaw policy"
golaw setup
golaw agent install --agent claude
golaw doctor
golaw scan --staged
```

init without --apply prints the complete deterministic diff and writes nothing. init --apply writes the policy, core templates and external enrollment record. A repeated init --apply produces no diff. init never arms git hooks. In a repository whose hooks lefthook installed before adoption, golaw hook passes silently until setup records its receipt.

The managed list in golaw.yml records kit-owned template paths and the hashes init wrote. Merged files such as Makefile and .editorconfig, and developer-owned files such as the baseline decision record, are not managed assets. The committed lefthook.yml lets teammates opt into the same rungs with golaw setup.

Commit the reviewed golaw.yml before agent install; init --apply ends with a line that asks for that commit while HEAD does not hold the policy it enrolled. On a branch that already has a commit, the policy in force is the golaw.yml at HEAD, so until a commit holds the file agent install refuses the enrolled repository and names this step, and scan exits 2, while an installed agent guard judges with the enrolled golaw.yml the index or the checkout holds, as long as it is the policy init enrolled, and otherwise denies with a reason that asks for the commit. On a new repository without a commit, the enrollment record carries the policy until the first commit.

init --apply never renders over a golaw.yml the checkout already holds: it keeps the reviewed policy and records managed template hashes without replacing its settings. It enrolls that policy. The one reviewed change init makes to such a policy is the additive capability proposal of golaw init --capabilities, described under Capabilities and policy-file limits. To start again from the defaults, remove the file first.

A committed change that changes the policy digest leaves the enrollment record behind: scan exits 2 and the guard denies until agent install enrolls the policy in force, printing the old and the new digest. The record follows a tightening only, so agent install and init --apply refuse to enroll a policy that weakens the enrolled one and name the floor.

The default policy is core only. The default human language is pt-BR. Findings, documentation, comments, and commits remain English. Repository identity never selects a profile implicitly.

The agent installer shows its changes, preserves unrelated settings in order, and records owned artifacts by hash. A repeated installation must produce no diff. User scope is the default.

## Choose local git rungs

Choose an absolute GOBIN and put it first on PATH before running golaw setup. Keep the pinned golaw executable on that PATH too. setup installs missing or mismatched pinned tools into the chosen GOBIN, verifies the versions that PATH resolves, and runs lefthook install. It records the resolved golaw path, tool versions and paths, effective hooks directory and installed hook hashes in its external receipt.

Before it installs anything, setup reads lefthook.yml and refuses one whose pre-commit, commit-msg or pre-push section runs no golaw hook of that rung, naming each rung it lacks. Only the run value of a command or job counts, with its YAML and shell quoting dropped, so `'"golaw" hook pre-commit'` routes the rung, and a fail_text, name or tag that only names golaw hook does not. init only proposes over a lefthook.yml the repository already holds, so apply that proposal first, then run golaw setup. setup reads lefthook.yml and the lefthook-local.yml beside it. A configuration that extends another file or pulls a remote one, and a command named through a YAML alias, are left to lefthook, which then arms what that configuration names, a stated limit.

The receipt is written last, after installation succeeds. An interrupted setup leaves no receipt; run golaw setup again to finish. A setup that fails after lefthook install, or that an interrupt or a termination signal stops there, puts every git hook back as it found it, a foreign hook lefthook moved aside included, and its reason says so, so only a setup that succeeds arms a rung. A kill that no process can catch is the stated limit: run golaw setup again to finish. A repeated setup with matching tools, routing and hooks changes nothing. To remove the owned git hooks and receipt while retaining installed tools, run:

```sh
golaw setup --uninstall
```

A git client started without GOBIN on its PATH, as a graphical client may be, cannot run golaw or lefthook from the armed hooks: the commit fails or the rung is skipped, a stated limit; doctor reports the golaw path setup recorded, and starting the client from a shell with GOBIN on PATH restores the rungs.

Each rung is a local choice in the checkout's golaw.yml. To turn all of them off:

```yaml
ladder:
  pre_commit: false
  commit_msg: false
  pre_push: false
  test_staged: false
```

Changing these switches causes no policy finding and needs no re-enrollment. A disabled rung returns silently before version checks or tool resolution. test_staged is off by default. Enabling it adds affected-package tests with the race detector to pre-commit; the full suite remains in remote CI or the platform pipeline. When a test of a package the commit touches fails, the rung runs the failing tests once more in a copy of the HEAD tree: a test that fails at HEAD too is named and left to the pipeline, while a test the commit breaks or adds is refused. A package that fails naming no test, as a TestMain that exits after its tests does, runs whole at HEAD, and the same unnamed failure there is named and left to the pipeline. A package that does not build still refuses, and the refusal names the off switch.

## Update the kit

An exact kit.version mismatch is refused in either direction. Install the pinned binary, or use the intended new version to review and apply an update:

```sh
golaw init --update
golaw init --update --apply
```

The preview writes nothing and shows the managed-file changes and exact version pin. Local edits are kept and reported. Apply writes the reviewed updates and matching enrollment record, preserving locally edited managed files. In a repository whose managed list records the kit templates, the update also writes the kit files a new version adds, each named in an `Arquivo novo do kit` line, and proposes the missing lines of a merged file such as the Makefile, and, in the modules of the baseline decision record, the modules of the kit's go.mod lines and tools/go.mod that no decision record lists, which policy.dependency-records measures since M3; you apply both as on adoption; a developer file and a repository that never adopted the templates are left alone. Stage every file listed in the reviewed update diff, exclude unrelated edits, and commit with the feat type so HEAD holds the updated policy. Keep the installed binary, kit.version and enrollment digests together. Run golaw setup again if you want the local rungs armed with the updated tool pins, then golaw doctor.

## Capabilities and policy-file limits

A capability names a shape of the repository and selects the checks and the templates that fit it: go for a Go module, cli for a command, api for a served contract, db for migrations and queries, hexagonal for the inner layers, image for a Dockerfile. api, cli, db and hexagonal each require go; image and go can stand alone. Unknown or repeated capabilities are invalid. A policy that breaks an implication is refused with the capability and the fix named. The capabilities in force are the reviewed capabilities line of golaw.yml and nothing else; nothing infers them from the files of the repository or from its remote.

For a new repository, name the capabilities on the first init:

```sh
golaw init --capabilities go,api,db,hexagonal
golaw init --capabilities go,api,db,hexagonal --apply
```

On an existing policy, the flag adds to the capabilities already enrolled. The preview writes nothing. It prints the diff of the capabilities line and the templates the new capabilities select, as a proposal over the reviewed policy. An explicit capability proposal adds to the existing set and never removes a capability. Apply writes the reviewed policy, matching enrollment and newly selected templates. Commit golaw.yml with the feat type, as on adoption. The flag beside --update is refused, because the update keeps the capabilities of golaw.yml: grow them first, commit, then update. doctor.capabilities reports the capabilities in force.

A check applies only where its capability holds. With go off, coverage and Go rung steps are not applicable. golaw coverage answers not applicable and exits 0, the pre-commit rung skips its Go steps, and every check that needs go answers NotApplicable. A disabled rung returns without a finding. The pre-push rung runs govulncheck only. M3 leaves migration, API-breaking and generation execution to their pipeline targets. The pre-commit rung names migrate-lint when a .sql file under a migrations directory is staged under db, openapi-breaking when openapi.yaml, openapi.yml or openapi.json is staged under api, and generate-check when a source of a generator configuration the index tracks is staged, and runs no tool for them.

The policy checks read each configuration through a declared subset, stated reader by reader in [rules.md](rules.md) with the rewrite that clears each limit. Outside the declared subset is Unmeasured, never a finding. The reason names the file, line and construct and ends with a pointer to that page. In scan and in the pipeline a required Unmeasured check exits 2. The pre-commit rung names that limit and continues unless another result refuses the commit. The remote gate of an adopting platform, and what a profile adds, is what [profiles.md](profiles.md) says.

## Read the result

doctor distinguishes artifact presence, matching hashes, executable resolution, enabled state, trust, validated agent version, and last dispatch evidence. Presence alone does not prove dispatch.

For git rungs, doctor reports tool versions, the setup receipt and the effective git hook routing. doctor reports managed drift: a managed file whose bytes differ from the hash init recorded, which golaw init --update shows and keeps, and a managed file gone from the checkout, which git restore --source=HEAD --staged --worktree -- <path> brings back.

The version line compares the host with the 2.1.295 floor and the validated 2.1.299, and reports a newer host as unknown until it is validated. A development build reports its pin as not enforced, as scan and the hook skip it. The final line gives the time and the OS of the last hook process the host ran for the repository, which proves dispatch but not trust.

A go install build reports version, module sum, and revision as metadata, with state source build, executable not verifiable.

scan exits 0 when clean, 1 for findings, and 2 for usage errors, internal failures, or a required check left unmeasured.

## Adopt a profile

A profile is optional and chosen by a person; core only is the default, and [profiles.md](profiles.md) describes the bundle, its schema and its evaluators. Install the bundle your platform names, adopt it in the repository and commit the pin:

```sh
golaw profile install <git url>@<tag>
golaw init --profile <name>
golaw init --profile <name> --apply
git add golaw.yml
git commit -m "feat: adopt the golaw profile"
golaw doctor
```

install verifies the bundle before it writes and prints `Perfil <name> <version> instalado de <url>@<tag>, digest <digest>`. init --profile previews the `profile:` line of golaw.yml from the installed stamp; under a profile that leaves destinations to the platform it names them after `Deixados para a plataforma:` and writes nothing there. doctor reports the pinned profile as `profile-installed` with its version, digest and the days since its date.

Every teammate installs the same bundle once with the command the pin names. A machine without it gets one error naming that command from scan, the armed rungs, init --apply, agent install, setup, coverage, config explain and the agent guard, never a silent core-only answer; a clone that machine never enrolled keeps getting the valid allow of the guard. Run the printed command in a terminal: until the bundle loads, the agent guards deny every call of the agent, the install among them.

A runner has no bundle until it installs one, and the kit provisions none. When the repository keeps the kit's CI lane, whose make ci runs golaw scan, the lane must run golaw profile install <source>@<version> with a credential that reads the bundle, before make ci, or every run refuses, the commit of the pin first; init names that step when it pins a profile that keeps the lane. A runner installs the bundle before golaw scan --enrolled too, whose optional second digest is the digest of the pin.

To move to a newer version, install its tag, run golaw init --profile <name> --apply, review a diff that changes only the version and the digest of the pin, and commit it, then run golaw init --apply once more to enroll the committed pin: until that run, scan, the armed rungs and the agent guards refuse the committed pin as an enrollment mismatch. To go back, run golaw profile rollback <name> and the same init, commit and second init. A teammate who pulls a moved pin installs the new version with the command the pin names, then runs golaw init --apply to enroll the pulled pin. golaw profile refresh <name> restores the current version from its source when its files changed, and golaw profile remove <name> deletes every installed copy. install, refresh, rollback and remove never change a pin.

## Inherited debt

On a repository that already holds findings, adopt with golaw init --seed-debt --apply instead of golaw init --apply. After a plain golaw init --apply, the refusal names how to start over before the first commit: delete golaw.yml and the enrollment record it names, then run golaw init --seed-debt --apply. It seeds one dated exception per finding that names a path, valid for 180 days, and writes docs/adr/0002-inherited-debt.md with every entry and why it exists; --coverage-profile <file> and --vulnerabilities <file> seed the coverage exemptions and the vulnerability waivers the same way. The debt set is seeded once: a second --seed-debt is refused, and later changes may remove an entry or bring its date closer, never add or broaden one, which scan --staged and the armed pre-commit rung refuse.

Adopting a profile after the first init seeds nothing: the profile's findings meet a debt set that is already committed. Fix them before committing the pin, or adopt the profile at the first init with golaw init --profile <name> --seed-debt --apply.

## Verify a release

Signed release assets are the primary distribution path with checksums, a Sigstore bundle, SBOMs, and attestations. Put the asset of your system, checksums.txt and checksums.txt.sigstore.json of the release in one directory, add the attestation of the asset, and run the downloaded binary's own verification before installing it:

```sh
gh attestation download golaw_<version>_<os>-<arch> --repo cristianargotti/golaw
./golaw_<version>_<os>-<arch> doctor --release .
```

gh attestation download writes sha256:<hex>.jsonl, or sha256-<hex>.jsonl on Windows, which doctor reads as it reads <asset>.sigstore.json. From a checkout of the kit, make release-verify BIN=<downloaded binary> DIST=<dir> runs the same command. A verified release prints release-verified with the signer and exits 0; any other result is red, names the file and the cause, and exits 1. A private Sigstore instance passes its roots.crt and rekor.pub with --trust-root <dir>.

doctor --release runs inside the binary it judges, so its verdict is only as honest as that binary. For a verification independent of it, verify checksums.txt and its bundle with cosign before running the executable. Pin the certificate identity to this repository's release.yml under refs/tags and to the exact selected tag. The issuer is https://token.actions.githubusercontent.com.

A release asset is named golaw_<version>_<os>-<arch>, with .exe on Windows. Install the verified asset on your PATH under the name golaw, golaw.exe on Windows, because the installed hook runs golaw from PATH.

## Enroll remote CI

A runner without a local enrollment record uses scan --enrolled <digests>, with expected policy and optional profile digests provisioned outside the checkout by a repository variable or pipeline setting.

The value is a 64-character lowercase hexadecimal policy digest, optionally followed by a comma and a 64-character lowercase hexadecimal profile digest. The supplied digests replace the local enrollment lookup for this scan. A missing policy at HEAD or a digest mismatch is refused.

After a person reviews the trusted policy, provision its expected digests outside the checkout. Do not derive the expected digests from the candidate checkout. Have the runner inject that external value as GOLAW_ENROLLED:

```sh
golaw scan --enrolled "$GOLAW_ENROLLED" .
```

## Local choice and removal

Local scans are optional aids. Installing the guard opts into its denials. Remove the owned integration with:

```sh
golaw agent uninstall --agent claude
```

Remote CI or the platform pipeline is the only hard gate. The full make ci runs there. Mutation, fuzz, benchmarks and integration suites run on demand.

If another guard exists, review the coexistence report before retiring it. Installation preserves unrelated entries and never silently disables another guard.

Read agent-contracts.md for evidence boundaries and security.md for recovery.

## Codex and Cursor agents

M4 adds the Codex CLI guard and the Cursor compatibility preview beside the Claude Code guard. Cursor's shell and read guards judge, while its writes are captured, not enforced. Each agent installation is its own opt-in: run only the agents you use, at the scope you want.

### Install and remove

Each command below is an alternative, not a step of a sequence. At user scope, the default:

```sh
golaw agent install --agent codex
golaw agent install --agent cursor
golaw agent install --agent all
golaw agent uninstall --agent codex
golaw agent uninstall --agent cursor
golaw agent uninstall --agent all
```

At project scope, from inside the repository:

```sh
golaw agent install --agent codex --scope project
golaw agent install --agent cursor --scope project
golaw agent install --agent all --scope project
golaw agent uninstall --agent codex --scope project
golaw agent uninstall --agent cursor --scope project
golaw agent uninstall --agent all --scope project
```

--scope user explicitly selects the default scope. Installation is a separate opt-in from golaw setup; neither initialization nor agent installation arms a git rung, and uninstall neither arms nor disarms one.

--agent all runs every agent of the registry in its order, claude, codex and cursor, each under a line `agent: <id>` with its own diff and receipt, and stops at the first agent whose installation fails, so the last agent named is the one to fix. golaw agent uninstall --agent all removes every agent that has a receipt at the scope and prints `not installed: <id>` for each one that has none.

### Where the artifacts go

| Artifact           | User scope                        | Project scope                   |
| ------------------ | --------------------------------- | ------------------------------- |
| Codex hooks        | `~/.codex/hooks.json`             | `.codex/hooks.json`             |
| Codex prefix rules | No user rules file                | `.codex/rules/golaw.rules`      |
| Codex marked block | `~/.codex/AGENTS.md`              | `AGENTS.md`                     |
| Cursor hooks       | `~/.cursor/hooks.json`            | `.cursor/hooks.json`            |
| Cursor rule        | No user rule file                 | `.cursor/rules/golaw.mdc`       |
| Shared skill       | `~/.agents/skills/golaw/SKILL.md` | `.agents/skills/golaw/SKILL.md` |

CODEX_HOME, when set, replaces ~/.codex for the Codex user artifacts and for its trust configuration, config.toml. It does not relocate the shared user skill. A CODEX_HOME reached through a symbolic link is named by its real path, the path Codex keys trust with, while the default ~/.codex is named as given. Project destinations are relative to the installation root, the root of the repository. The marked block points at the user skill and, for a project installation, at .agents/skills/golaw/SKILL.md in that repository, so one block serves both scopes; Codex and Cursor read the same skill file, which each receipt lists.

Codex prefix rules are written only with --scope project; a user-scope rules file would apply to every Codex session, enrolled or not. In a repository golaw never enrolled the hook gives a valid allow, so a user rules file would deny there what the hook allows: a user-scope installation writes no rules file and leaves a personal file of that name untouched. No user-scope rule file is installed for the compatibility preview. Cursor documents no user rules file, so its rule exists at project scope only.

Codex reads the project hooks of a linked git worktree from the .codex folder of the root checkout, so install at project scope from the root checkout. An agent file that is itself a symbolic link, as a dotfiles checkout may leave, is refused and never followed, because following it would write outside the agent's directory.

### What install and uninstall change

The installer computes every change first and shows the diff before it writes; a repeated installation finds nothing to change and writes nothing. golaw's handlers are appended after the foreign ones of each event and an owned handler is replaced in place, so foreign hook entries keep their order; the marked block, from `<!-- golaw:begin -->` to `<!-- golaw:end -->`, is replaced in place or appended. When a change moves a Codex handler, golaw's or a foreign one, to another position, the install or the uninstall names its old and new trust identity, and that handler needs review in /hooks again, because Codex keeps the approval under the old key.

One receipt per agent and scope records what the installation owns: the hash of every file it owns whole, the exact JSON of each handler it owns with its event and position, and the hash of each marked block it wrote into a file you own. Its path comes from agents.ReceiptPath, `<state>/<agent>/<scope>/receipt.json`, with the project identity as one more directory at project scope. The state directory comes from policy.DefaultDirs: XDG_STATE_HOME when set, otherwise LOCALAPPDATA on Windows and ~/.local/state on macOS and Linux, with the golaw suffix.

Uninstall reads the receipt and removes exactly what it lists: the owned handlers, matched by their exact definition, the marked block, the rules file, the Cursor rule and the shared skill, then the receipt. Foreign entries and the rest of a file you own stay, and a hooks file or AGENTS.md the installation created is removed only when nothing else is left in it. Uninstall keeps a shared skill while another receipt owns it. A changed owned file is refused with ErrChanged and is not removed. Without a receipt nothing is guessed. Uninstall leaves golaw.yml and the enrollment as they are.

### Codex trust

Codex runs a hook only when its feature is on, its project layer is loaded and the hash of its definition is trusted. After an install:

1. Codex uses features.hooks in config.toml, on by default; enable it if it is disabled.
2. With --scope project, trust the project in Codex, which loads the .codex folder of a project only once the project is trusted. Project trust and hook trust are separate steps.
3. Open /hooks in Codex and approve the golaw hooks whenever the install prints that step.
4. Run golaw doctor again, with --scope project for a project installation.

The installer never writes hooks.state. Codex keeps the trust of each handler in the hooks.state table of its config.toml, keyed by the hooks file, the event and the position, and the installer only reads it to decide whether to print the /hooks step. A changed command, matcher, timeout or handler position can require review even when artifact installation succeeded, and Codex skips an untrusted or changed hook without a word, which is why doctor reports trust.

On macOS and Linux each hook command is the quoted absolute path of the golaw binary that ran the install, followed by hook codex or hook cursor. On Windows, the Codex commandWindows and the Cursor command are an encoded PowerShell launcher that passes stdin to the binary byte for byte and exits with its status. Windows acceptance exercises both command processors, cmd.exe /c and powershell.exe -Command. That is offline evidence of the launcher, never live host evidence. After the binary moves, run agent install again; for Codex the changed command needs /hooks review.

### Read doctor

Installed, trusted and active are separate facts. golaw doctor prints one report per agent for the scope it inspects, user by default or --scope project, with a pt-BR summary after the lines of each agent. doctor --json prints one JSON document per agent. doctor exits 0 after printing a report, including findings; collection failure exits 2. Exit 0 says the report was collected, not that a guard protects: read the state line.

| Line                | Codes                                                                                           | Meaning                                                                                                                     |
| ------------------- | ----------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| doctor.installed    | installed, installed-drift, not-installed                                                       | the receipt exists and every artifact, entry and block it lists is present and matches                                      |
| doctor.binary       | binary-matches, binary-differs, binary-unresolved                                               | the binary each hook command names, against the running one                                                                 |
| doctor.trusted      | trusted, untrusted, revalidation-required, trust-unknown                                        | whether the host will run the hook                                                                                          |
| doctor.dispatch     | observed, stale, unobserved                                                                     | the current definition's dispatch evidence: the host ran it for this repository, ran an earlier definition, or never ran it |
| doctor.state        | protecting, installed-untrusted, trusted-inactive, not-installed, trust-unknown, preview-active | the one line that tells the three facts apart                                                                               |
| doctor.fail-mode    | fail-closed, host-fails-open                                                                    | per event, whether the host blocks the tool when the hook fails                                                             |
| doctor.version      | agent-version-validated, agent-version-unvalidated, agent-version-unavailable                   | Codex: codex --version against the pinned patch grammar                                                                     |
| doctor.agents-chain | agents-chain-within, agents-chain-over, agents-chain-unread                                     | Codex: the AGENTS.md at the repository root against the bytes Codex reads of the chain                                      |
| doctor.preview      | cursor-writes-not-enforced                                                                      | Cursor only, always: writes not enforced, payload unknown                                                                   |
| doctor.capture      | capture-recorded, capture-absent                                                                | per Cursor write event, the key names a payload carried, never a value                                                      |
| doctor.duplicate    | no-duplicate, duplicate-hook, skill-duplicate                                                   | a second copy of the hook or the skill the agent would load                                                                 |
| doctor.companion    | companion-absent, companion-found                                                               | the companion gate in the agent's hooks file, before or after golaw                                                         |

protecting requires matching installation, established trust and current observed dispatch. An agent with no receipt at a scope is not-installed in state unknown, never red. The Claude report keeps the M1 lines of its guard unchanged, so with no Claude receipt at the scope doctor.artifacts, doctor.plugin, doctor.read, doctor.hooks and doctor.pin stay red as M1 printed them before an installation; its doctor.state line says not-installed in state unknown, and if you do not use Claude Code those lines ask for nothing. An installation that drifted, or a receipt that cannot be read, is installed-drift, and its state is not-installed in state red with the install command that repairs it. An installed hook the host will not run is installed-untrusted; one whose trust doctor cannot read is trust-unknown; a trusted one that has not run with the current definition is trusted-inactive. Cursor is never protecting: an installed, trusted and observed Cursor hook is preview-active, because its writes are captured, not enforced.

Codex trust comes first from codex app-server hooks/list, when the installed CLI answers within the doctor's deadline, and otherwise from the fallback that compares each receipt definition's hash with the record in config.toml; the line names its source. When config.toml turns features.hooks off, Codex lists no hook at all, so doctor reads the record, which names the feature as the cause, without starting codex app-server. Starting codex app-server in your CODEX_HOME lets Codex create its own state there, as any Codex start does; security.md lists what Codex 0.163.0 creates. A handler changed since it was trusted is revalidation-required; one never trusted, turned off, under a disabled hooks feature or in a project Codex does not trust is untrusted. Claude plugin trust remains trust-unknown without suitable live evidence, even after the hook ran. Cursor runs user hooks without a trust step, so its user trust is trusted, and runs project hooks only in a trusted workspace doctor cannot read, so its project trust is unknown. Cursor user trust does not turn captured writes into enforced writes. Capture does not turn on write enforcement.

Claude's onFailure: block and Cursor's failClosed: true are fail-closed. Codex has no fail-closed setting, so its line is host-fails-open in state unknown, its stated limit; a Claude or Cursor handler without its blocking setting is host-fails-open in state red. The version line is agent-version-validated when Codex answers the version its patch grammar is pinned to; Claude keeps its floor line, and Cursor, with no validated version, has none.

Cursor imports Claude settings-file hooks by default and maps Edit to Write and Bash to Shell, so a golaw hook command in ~/.claude/settings.json would run twice; the hooks live in the plugin, and doctor reports duplicate-hook. A golaw skill under ~/.codex/skills or ~/.cursor/skills beside ~/.agents/skills produces skill-duplicate. Remove the extra copy in either case.

Codex stops reading the AGENTS.md chain at 32 KiB and the golaw block stays under 1,000 bytes; doctor.agents-chain measures the AGENTS.md at the repository root against that budget. The chain of every session starts at that file, or at AGENTS.override.md when the root holds one, which Codex reads in its place, so a root file over the budget is certain for every session: the line is red with the file and its size, and Codex drops the rest of the file, the golaw block included when install appended it there. A project_doc_max_bytes that the Codex configuration of the person or of the project sets replaces 32 KiB; when both set one, doctor measures against the larger, so a budget it cannot pin never makes the line red. The files below the root, which differ per session, a fallback file name, a budget given on the command line or in a profile, and a project root other than the git root are stated limits doctor does not measure. Keep your own AGENTS.md files within that budget.

doctor.companion finds a companion gate by its neutral names: a handler whose command runs standards_gate.py in the agent's hooks file, reported before or after golaw's handler of the same event; for Claude the coexistence report also reads its registry, standards-gate.json. No file may receive two blocking verdicts: retire from the companion gate each family golaw owns before both run.

## Starting states golaw does not support

Each starting state below is a stated limit of this release, with what the person or the platform does about it.

When CODEOWNERS gives `.github/` to a team other than the catch-all owner, init writes no Actions file and no Dependabot file there, so the generated CI and the Dependabot cooldown stay out of the repository, and the remote gate is whatever the platform runs. The team that owns `.github/` adds a stage that runs `make ci`, or the owners move the path.

A container test that needs a private registry or a cloud login is outside the kit: the `test` target of `golaw.mk` runs `go test` on the machine and needs neither. Run the container suite on demand.

A Swagger 2.0 tree a generator wrote is inherited state the kit does not regenerate: the API path of the kit is OpenAPI 3 with oapi-codegen, and `policy.openapi-assertions` judges no Swagger 2.0 document and measures clean until the OpenAPI 3 contract `redocly.yaml` names exists. Writing that contract brings the API under the check.

A go line past Go's support window adopts, and its standard library advisories close only through a go line bump, so the `vuln` target and the push rung need dated entries in `vulnerability_waivers` until the line moves; the kit never moves it.

A repository born with lint debt is red on its first `make ci` until the debt is fixed: M3 ships no lint baseline, and the reviewed debt set that only `init --apply` seeds arrives with the inventory pass of a later release. Fix the debt, or keep `make ci` off the gate until then; the pre-commit rung reports only the lint a commit introduces.

A generator that rewrites a file without the Go generated header flips with `make fmt`; naming such files is profile data of a later release. Commit the generator's output after `make fmt`.
