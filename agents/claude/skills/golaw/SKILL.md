---
name: golaw
description: Use for Go implementation, tests, refactoring, review, dependencies, and commits in a repository enrolled in golaw.
---

# Go engineering with golaw

Read golaw.yml and the rules of the kit, https://github.com/cristianargotti/golaw/blob/v0.3.0/docs/rules.md, before changing the repository.

Reuse in this order: existing repository code, the standard library, an existing platform capability, an installed dependency, then the smallest implementation that meets the need. Preserve boundary validation, error handling, and security. Record every new direct dependency and its alternatives in the configured decisions directory.

Follow the strongest documented practice. Existing weak code is not a reason to repeat it.

Keep one use case per file, named verb_object.go with its matching test file. Give it a constructor and an Execute method. Keep transport and persistence concerns outside it.

Declare package failure modes as sentinels in errors.go. Begin an operation with wrap := failures.Prefix("package: action") and wrap each error returned across the package boundary. When at least three functions share a prefix, keep the shared wrapper in errors.go. Do not repeat a layer in an error chain. Classify failures with errors.Is against sentinels, never by comparing error text. At an HTTP boundary, map those sentinels to the contract's response.

Keep a pull request within 400 changed lines and one topic. Combine topics only when the owner explicitly requests the bundle. Ask before pushing or opening a pull request.

Explain why in English comments. Explain why the current code needs the decision. Keep documentation and conventional commits in English, with no commit trailers. Use the configured language for strings addressed to people, pt-BR by default.

Pass time through a Clock port and compose a UTC clock at the entry point. A metric belongs to the package that emits it.

Use the repository's integration test conventions and declared dependencies. Keep integration suites on demand. For queries over growing tables, inspect the execution plan with realistic data.

Keep applied migrations immutable. Verify rollback and reapplication in an isolated database, including the empty starting state. Review data preservation and recovery before delivery.

Write a failing test first, observe its failure, implement the smallest complete change, and observe it passing. Record the command and result for each verification claim. Report anything unmeasured explicitly.

Policy-file checks run in scan and staged scan, not in agent events. Checks apply only with their required capabilities; a missing capability makes a check inapplicable. Read configuration only through each reader's declared subset. Outside the declared subset is Unmeasured, never a finding. A stated limit names the file, line and construct the verdict depends on; unrelated unsupported syntax is skipped. Required Unmeasured results fail scan and remote CI. An armed pre-commit rung names that limit and leaves it to the pipeline. That limit alone does not refuse the commit and is not clean evidence; it does not excuse other failures.

Run golaw scan . before finishing. Run golaw scan --staged before finishing staged work. Local checks are optional aids. Remote CI or the platform pipeline is the only hard gate.

Run golaw lint --surface before finishing when you changed a non-Go file. Move an inline script into a file, and export the code a test needs instead of evaluating it.

Only the developer arms git rungs with golaw setup; init alone never arms them. Each rung switches off in golaw.yml without a finding. Armed rungs finish in seconds, and pre-push runs govulncheck only. The full make ci belongs in remote CI or the platform pipeline; mutation, fuzzing, benchmarks and integration stay on demand.

Installed guards deny recognized violations only on enforced routes. Capture, feedback and installation alone do not prove enforcement. Do not weaken policy to bypass a finding.

Use golaw doctor to distinguish installed, trusted and active hooks. Read docs/agent-contracts.md and docs/security.md of the kit at the same tag before relying on an integration.

Cursor is a compatibility preview. Write and Delete payloads are captured by key name and allowed. Post feedback is advisory.

After a Codex hook definition changes, review it through /hooks. Never write hooks.state yourself.

The rules page carries the prose mapping. Read docs/checklists.md of the kit at the same tag. Use its conditional checks and evidence record, and docs/context-template.md for hexagonal context shapes.
