# Setup

M2 provides worktree and staged scanning, core templates, optional git rungs, and the Claude Code guard. M3 adds the capabilities and the policy-file checks, described below and in [rules.md](rules.md). Other agent adapters belong to later milestones; a warning about the chain of agent instruction files concerns that later adapter integration, not a new installation of M3.

Adopting repositories need Go 1.24 or newer. Building the kit needs Go 1.27. Claude Code 2.1.295 is the minimum host version; 2.1.299 is validated.

## Install and enroll

golaw setup is optional. Include that step only when you want local git rungs; agent install separately opts into the agent guard.

The adoption commit stages every file listed in the reviewed init diff. That includes golaw.yml and the generated templates. The adoption commit and every init --update commit use the feat type, because commit.shape holds a commit that is not feat to 40 new files and 400 new lines. The rendered templates exceed those limits.

The sequence below shows the default core paths. Adjust the staging list to the actual diff, including an existing decisions directory selected by init, and omit paths that init did not change. Exclude unrelated edits from the adoption commit; when a file also holds unrelated edits, stage only the reviewed adoption hunks.

init distinguishes writes from proposals for existing files. init --apply leaves proposed bytes untouched. Review and apply the accepted proposal hunks yourself before staging, including the include of golaw.mk in an existing Makefile. Unapplied proposals can be displayed again on a repeated init.

```sh
go install github.com/cristianargotti/golaw/cmd/golaw@v0.2.0
golaw init
golaw init --apply
git add golaw.yml .editorconfig .gitattributes .github/dependabot.yml .gitleaks.toml .golangci.yml .gremlins.yaml .mockery.yaml Makefile golaw.mk lefthook.yml test/deadcode/reachability_test.go tools/go.mod docs/adr/0001-inherited-baseline.md
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

The preview writes nothing and shows the managed-file changes and exact version pin. Local edits are kept and reported. Apply writes the reviewed updates and matching enrollment record, preserving locally edited managed files. In a repository whose managed list records the kit templates, the update also writes the kit files a new version adds, each named in an `Arquivo novo do kit` line, and proposes the missing lines of a merged file such as the Makefile, which you apply as on adoption; a developer file and a repository that never adopted the templates are left alone. Stage every file listed in the reviewed update diff, exclude unrelated edits, and commit with the feat type so HEAD holds the updated policy. Keep the installed binary, kit.version and enrollment digests together. Run golaw setup again if you want the local rungs armed with the updated tool pins, then golaw doctor.

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

Signed release assets are the primary distribution path with checksums, a Sigstore bundle, SBOMs, and attestations. Verify checksums.txt and the accompanying bundle yourself; automatic release verification in doctor arrives in M5.

For an independently downloaded release, verify checksums and the bundle before running the executable. Pin the certificate identity to this repository's release.yml under refs/tags and to the exact selected tag. The issuer is https://token.actions.githubusercontent.com.

A release asset is named golaw_<version>_<os>-<arch>, with .exe on Windows. Install the verified asset on your PATH under the name golaw, golaw.exe on Windows, because the installed hook runs golaw from PATH.

scan exits 0 when clean, 1 for findings, and 2 for usage errors, internal failures, or a required check left unmeasured.

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
