---
record: 1
title: cobra for the command tree
date: 2026-10-14
status: accepted
modules:
  - github.com/spf13/cobra
  - github.com/spf13/pflag
---

# 1. cobra for the command tree

## Context

The kit is one binary with a command tree (`scan`, `hook`, `init`, `agent`, `doctor`, `coverage`, `version`) whose subcommands take flags, print help and map exit codes. The flag package of the standard library has no subcommands, no completion and no generated help.

## Decision

Use `github.com/spf13/cobra`, with its flag library `github.com/spf13/pflag`, at an exact pinned version, wired only in `cmd/golaw`; every other package stays free of it. Exit codes are decided in `cmd/golaw` from the engine verdict: 0 clean, 1 findings, 2 usage, failure or a required check left unmeasured.

## Consequences

Two modules enter the binary. The versions are pinned exactly and enter seven days after publication; a bump is a reviewed diff with the changelog line that explains it.
