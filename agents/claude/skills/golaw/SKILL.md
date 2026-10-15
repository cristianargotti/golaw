---
name: golaw
description: Apply the repository's Go engineering standard while changing code.
---

# Go engineering standard

Read the policy and relevant code before choosing a design.
Preserve existing contracts and make the smallest complete change.

## Reuse and design

Reuse local code before extracting another abstraction.
Prefer the standard library, then existing dependencies, then a new dependency.
Record the reason for each new direct dependency in the decisions directory.
Keep one use case per file and responsibilities explicit.
Working code is the floor; clarity and careful verification define quality.

## Errors and time

Use failures.Prefix where the repository provides that convention.
Wrap causes with %w and compare sentinels with errors.Is.
Do not turn err.Error() into application data.
Pass time through the application's Clock port.
Keep metric definitions and recording at the boundary owning the operation.

## Tests and migrations

Follow the repository's integration test setup and cleanup conventions.
Exercise the real boundary when a unit double cannot establish the contract.
Keep applied migrations immutable and new migrations ordered.
Review data preservation, compatibility and recovery before delivery.

## Delivery

Keep each pull request focused on one topic and small enough to review.
Read docs/checklists.md when it exists.
Ask before pushing or opening a pull request.
Run golaw scan . before finishing.
When staged scanning is available, run golaw scan --staged before you finish.
Report the verification performed and the evidence still absent.
