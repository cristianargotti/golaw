# golaw

golaw puts a Go repository under one engineering standard and keeps it there: from inside the coding agents (Claude Code first, Codex CLI and Cursor later), from the git hooks a developer chooses to arm, and from CI, with one binary, one skill and a set of templates.

Status: pre-release. M2 adds staged scans, core adoption templates, pinned tool setup, optional pre-commit, commit-msg and pre-push rungs, and managed updates. The M1 Claude Code guard remains available. Native rung and live agent evidence is documented separately in docs/agent-contracts.md.

## Install

Build the pinned release from source, never from latest:

```sh
go install github.com/cristianargotti/golaw/cmd/golaw@v0.2.0
```

When using a signed release asset, select the exact version and system, for example `golaw_0.2.0_<os>-<arch>` (with `.exe` on Windows), verify it against `checksums.txt` and its Sigstore bundle as docs/setup.md explains, and put it on your PATH under the name golaw (`golaw.exe` on Windows): the Claude Code plugin runs `golaw` from PATH. A source build reports `source build, executable not verifiable`; automatic release verification in doctor arrives in M5.

## Adopt a repository

The M2 sequence is explained step by step in docs/setup.md. `golaw setup` is optional and is the action that arms local git hooks; init never arms them. Remote CI or the platform pipeline is the only hard gate.

Review the complete init diff before applying it. The adoption commit stages every file listed in that reviewed diff, including the policy and generated templates, and excludes unrelated edits. This example uses `docs/adr` as the configured decisions directory; adjust the paths to the actual diff. The managed list records kit-owned template hashes, and committed `lefthook.yml` lets teammates opt into the same rungs with `golaw setup`.

```sh
golaw init
golaw init --apply
go get github.com/quasilyte/go-ruleguard/dsl@v0.3.23
go mod tidy
git add golaw.yml .editorconfig .gitattributes .github/dependabot.yml \
  .github/workflows/ci.yml .gitleaks.toml .golangci.yml .gremlins.yaml \
  .mockery.yaml Makefile golaw.mk lefthook.yml scripts/lint/rules.go \
  test/deadcode/reachability_test.go tools/go.mod go.mod go.sum \
  docs/adr/0001-inherited-baseline.md
git commit -m "feat: adopt the golaw policy"
```

The adoption commit and every `init --update` commit use `feat`, because the commit shape limits of 40 new files and 400 new lines apply to other types. Run setup only if you choose to arm the local rungs:

```sh
golaw setup
```

Then install the agent guard and inspect the result:

```sh
golaw agent install --agent claude
golaw doctor
golaw scan --staged
```

## More

- Setup, results and removal: `docs/setup.md`.
- Security model and recovery: `docs/security.md`.
- Agent contracts and evidence: `docs/agent-contracts.md`.
- The laws of this repository: `CONTRIBUTING.md`.
- Security policy: `SECURITY.md`.
- Decision records: `docs/adr/`.
