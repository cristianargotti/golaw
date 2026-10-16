# golaw

golaw puts a Go repository under one engineering standard and keeps it there: from inside the coding agents (Claude Code first, Codex CLI and Cursor later), from the git hooks a developer chooses to arm, and from CI, with one binary, one skill and a set of templates.

Status: pre-release. The first milestone, M1, ships the Claude Code guard over a scanned tree, and v0.1.0 is its first tag.

## Install

Build the pinned release from source, never from latest:

```sh
go install github.com/cristianargotti/golaw/cmd/golaw@v0.1.0
```

Or take the signed release asset for your system, named `golaw_0.1.0_<os>-<arch>` (with `.exe` on Windows), verify it against `checksums.txt` and its Sigstore bundle as docs/setup.md explains, and put it on your PATH under the name golaw (`golaw.exe` on Windows): the Claude Code plugin runs `golaw` from PATH.

## Adopt a repository

The M1 sequence, step by step in docs/setup.md:

```sh
golaw init
golaw init --apply
git add golaw.yml
git commit -m "build: adopt the golaw policy"
golaw agent install --agent claude
golaw doctor
golaw scan .
```

## More

- Setup, results and removal: `docs/setup.md`.
- Security model and recovery: `docs/security.md`.
- Agent contracts and evidence: `docs/agent-contracts.md`.
- The laws of this repository: `CONTRIBUTING.md`.
- Security policy: `SECURITY.md`.
- Decision records: `docs/adr/`.
