---
name: golaw
description: Use for Go implementation, tests, refactoring, review, dependencies, and commits in a repository enrolled in golaw.
---

# Go engineering with golaw

Read golaw.yml and the rules of the kit, https://github.com/cristianargotti/golaw/blob/v0.1.0/docs/rules.md, before changing the repository.

Reuse in this order: existing repository code, the standard library, an existing platform capability, an installed dependency, then the smallest implementation that meets the need. Preserve boundary validation, error handling, and security. Record every new direct dependency and its alternatives in the configured decisions directory.

Follow the strongest documented practice. Existing weak code is not a reason to repeat it.

Keep one use case per file, named verb_object.go with its matching test file. Give it a constructor and an Execute method. Keep transport and persistence concerns outside it.

Declare package failure modes as sentinels in errors.go. Begin an operation with wrap := failures.Prefix("package: action") and wrap each error returned across the package boundary. When at least three functions share a prefix, keep the shared wrapper in errors.go. Do not repeat a layer in an error chain. Classify failures with errors.Is against sentinels, never by comparing error text.

Keep a pull request within 400 changed lines and one topic. Combine topics only when the owner explicitly requests the bundle. Ask before pushing or opening a pull request.

Explain why in English comments. Keep documentation and conventional commits in English, with no commit trailers. Use the configured language for strings addressed to people, pt-BR by default.

Pass time through a Clock port and compose a UTC clock at the entry point. A metric belongs to the package that emits it.

Use the repository's integration test conventions and declared dependencies. Keep integration suites on demand. For queries over growing tables, inspect the execution plan with realistic data.

Keep applied migrations immutable. Verify rollback and reapplication in an isolated database, including the empty starting state. Review data preservation and recovery before delivery.

Write a failing test first, observe its failure, implement the smallest complete change, and observe it passing. Record the command and result for each verification claim. Report anything unmeasured explicitly.

Run golaw scan . before finishing. When golaw scan --staged exists in your version, run it before finishing staged work. Local checks are optional aids. Remote CI or the platform pipeline is the only hard gate.

Installed agent guards deny violations until a person uninstalls them. Do not weaken policy to bypass a finding.

The rules page carries the prose mapping; read docs/checklists.md of the kit at the same tag when it exists.
