# M1 setup

M1 provides worktree scanning and the Claude Code guard. Git rungs, staged scanning, general template installation, and other agent adapters belong to later milestones.

Adopting repositories need Go 1.24 or newer. Building the kit needs Go 1.27. Claude Code 2.1.295 is the minimum host version; 2.1.299 is validated.

## Install and enroll

Run the pinned M1 sequence:

```sh
go install github.com/cristianargotti/golaw/cmd/golaw@v0.1.0
golaw init
golaw init --apply
git add golaw.yml
git commit -m "build: adopt the golaw policy"
golaw agent install --agent claude
golaw doctor
golaw scan .
```

init without --apply prints the complete deterministic diff and writes nothing. M1 init --apply writes golaw.yml and the external enrollment record, with no templates. A repeated apply produces no diff. init never arms git hooks.

Commit the reviewed golaw.yml before agent install. On a branch that already has a commit, the policy in force is the golaw.yml at HEAD, so until a commit holds the file agent install refuses the enrolled repository and names this step, and scan exits 2. On a new repository without a commit, the enrollment record carries the policy until the first commit.

init --apply never renders over a golaw.yml the checkout already holds: it keeps the file, as reviewed by its person or committed by a teammate, and enrolls it. To start again from the defaults, remove the file first.

A committed change of golaw.yml leaves the enrollment record behind: scan exits 2 and the guard denies until agent install enrolls the policy in force, printing the old and the new digest. The record follows a tightening only, so agent install and init --apply refuse to enroll a policy that weakens the enrolled one and name the floor.

The default policy is core only. The default human language is pt-BR. Findings, documentation, comments, and commits remain English. Repository identity never selects a profile implicitly.

The installer shows its changes, preserves unrelated settings in order, and records owned artifacts by hash. A repeated installation must produce no diff. User scope is the default.

## Read the result

doctor distinguishes artifact presence, matching hashes, executable resolution, enabled state, trust, validated agent version, and last dispatch evidence. Presence alone does not prove dispatch.

A go install build reports version, module sum, and revision as metadata, with state source build, executable not verifiable.

Signed release assets are the primary distribution path with checksums, a Sigstore bundle, SBOMs, and attestations. M1 publishes those artifacts; automatic release verification in doctor arrives in M5.

For an independently downloaded release, verify checksums and the bundle before running the executable. Pin the certificate identity to this repository's release.yml under refs/tags and to the exact selected tag. The issuer is https://token.actions.githubusercontent.com.

scan exits 0 when clean, 1 for findings, and 2 for usage errors, internal failures, or a required check left unmeasured.

## Local choice and removal

Local scans are optional aids. Installing the guard opts into its denials. Remove the owned integration with:

```sh
golaw agent uninstall --agent claude
```

Remote CI or the platform pipeline is the only hard gate. M1 does not provide golaw setup or scan --staged.

If another guard exists, review the coexistence report before retiring it. Installation preserves unrelated entries and never silently disables another guard.

Read agent-contracts.md for evidence boundaries and security.md for recovery.
