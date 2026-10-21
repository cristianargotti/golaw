---
record: 5
title: the ruleguard dsl for the byte cut rule
date: 2026-10-20
status: accepted
modules:
  - github.com/quasilyte/go-ruleguard/dsl
---

# 5. the ruleguard dsl for the byte cut rule

## Context

The standard forbids cutting a string by a byte count, because the cut can split a multibyte rune. No linter golangci-lint ships carries that rule; gocritic runs it through ruleguard from `scripts/lint/rules.go`, which `policy.ruleguard` requires of every Go repository, the kit included. Ruleguard type-checks the rules file against `github.com/quasilyte/go-ruleguard/dsl` inside the module it lints, so the module must be required by `go.mod`; without it the lint stops with an import error.

## Decision

Require `github.com/quasilyte/go-ruleguard/dsl` at an exact pinned version. Only `scripts/lint/rules.go` imports it, behind the `ruleguard` build tag, so no package of the binary compiles it.

## Consequences

The module enters `go.mod` and `go.sum` and no build of the kit. The version enters seven days after publication. An adopter receives the same requirement as a proposal of `golaw init`, never as a silent write.
