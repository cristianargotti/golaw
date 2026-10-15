# Security policy

## Supported versions

The latest release receives fixes. Older releases are not patched; upgrade with `go install github.com/cristianargotti/golaw/cmd/golaw@<version>` or with the signed release asset.

## Reporting a vulnerability

Use GitHub private vulnerability reporting on this repository (Security tab, "Report a vulnerability"). Do not open a public issue for a security problem. Reports are acknowledged within five business days.

## What the kit protects and what it cannot

The assurance levels per agent and operating system, what each guard stops and cannot stop, and the recovery procedure when the hook itself is the failure are documented in `docs/security.md`. Remote CI or the platform pipeline is the only hard gate; the local rungs and the installed agent guards are opt-in aids that a developer arms and can remove.
