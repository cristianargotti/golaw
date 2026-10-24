# Engineering checklists

Use these lists to plan a change and judge its evidence. Read the [rules](rules.md) and, for hexagonal Go code, the [context guide](context-template.md). Apply the strongest documented practice; an existing weakness is not a reason to repeat it.

Every local check is optional. Remote CI or the platform pipeline is the only gate. The full `make ci` belongs there. Only the developer chooses to arm git rungs with `golaw setup`; `init` alone never arms them. Each rung can be switched off in `golaw.yml` without a finding. Armed rungs finish in seconds, and pre-push runs govulncheck only.

## Builder

### Before changing code

- Read the destination repository's branch, title and body conventions. Read its policy, ownership rules, decision records and delivery template before choosing paths or proposing a change.
- Define the behavior, failure modes and input boundaries. Name the expected result for empty, malformed, duplicate, concurrent and cancelled input where relevant.
- Climb the reuse ladder: existing repository code, the standard library, an existing platform capability, an installed dependency, then the smallest complete implementation. Preserve boundary validation, error handling and security at every step.
- Identify the capabilities in force and the concerns the change actually touches. Use the conditional checks below only where those concerns exist; neither repository identity nor a remote name selects a profile.
- A check needs a red case, a near miss and an explicit input universe. Declare the reader's subset and the evidence needed for its verdict before implementing the check.
- Each managed object names its property, count, policy family, alternatives and reversal condition. Put the decision and its cost in the repository's decision records, including why an existing object cannot serve it.
- Record every new direct dependency by exact module path in the configured decisions directory. Explain the need, alternatives, version and publication-age evidence.

### While implementing

- Write the failing test first and observe the intended failure. Implement the smallest complete change, then observe it passing. A failure to compile for an unrelated reason does not prove the behavior.
- Bind a new check's red and near-miss fixtures through its proof manifest. Verify that removing the check loses detection and that making it always report a finding rejects the near miss. Preserve existing behavior and applicability.
- Reuse established types, helpers and error identities. Keep source files within 300 lines and functions within 40 lines under funlen semantics with comments ignored; test functions lose the function limit, not the file limit.
- Preserve generated-source ownership: edit the contract, query or generator input, regenerate, and keep generated outputs versioned and fresh. Do not hand-edit generated code or overwrite another owner's files.
- Keep comments in English and explain why. Keep documentation and commits in English, human-channel strings in the configured language, and typographic dashes, private names and personal paths out of the public tree.
- Read both the exit code and stderr. Test harnesses must preserve failure status and diagnostic evidence instead of treating empty stdout as success.
- Use benchmarks for wall-clock budgets. Functional tests assert results, cancellation and bounded work without a workstation timing threshold.
- Run only one heavy process at a time on the workstation. Mutation, fuzzing, benchmarks and integration suites remain on demand.

### Before delivery

- Keep the change on one topic and within the destination's size rules. Review the whole diff, including generated outputs, deletions, documentation and configuration applicability.
- Collect the relevant test, race, lint, format, scan, coverage and generated-freshness evidence. A local run is a choice; remote evidence can satisfy the record. Report the exact commands, results and anything unmeasured under Evidence.
- Include a suite result under `GOARCH=amd64` from an amd64 runner or an environment able to execute its binaries; cross-compilation alone does not prove the suite ran.
- Keep the final commit conventional, in English, with body lines at most 100 characters and no trailer. Check the author address and final squash message for an added trailer. Preserve the configured identity and obtain the owner's authorization before a push or pull request.
- A repeated finding becomes a check when a meaningful red case and near miss exist. Record a lesson the same day and date its change with a reason. A rule change belongs in `CHANGELOG.md` with that date and reason.

## Reviewer

Use tools for verification and one reviewer for judgment. Judge the behavior against its requirement and evidence. A clean review does not require inventing a finding; an intent or quality judgment is not automatically a machine check.

- Inspect the changed boundary cases and the near misses that must remain valid. A false finding or false denial is a defect. A refusal must identify a recognized violation and a repair the reader can apply without help.
- Check the configuration actually in force, the capabilities and the reader's declared subset. Do not turn unsupported syntax or absent evidence into a policy violation, and do not broaden a reader just to cover an exotic construct.
- Inspect error identity across packages, wrapping and `errors.Is` against sentinels. Ensure diagnostic text is not persisted as business data or exposed as a raw API error.
- For applicable database and network changes, inspect query plans, transaction boundaries, retry limits and deadline budgets. Check cancellation, concurrent delivery, permanent failure and recovery, not only the happy path.
- Inspect security boundaries, generated-source freshness and ownership, dependency decision records, version pins and the final commit and squash messages.
- Check coverage against the full build universe, not only the touched packages. Require 80 percent total, per package and per file, with the 20 statement minimum applying to package and file floors and no kit exemptions. Account for generated files and mocks separately and identify missing profile data rather than claiming zero coverage or success from it.
- Two tools must not disagree silently. Name the shared input, reproduce the disagreement and resolve which rule and measurement each tool implements before relying on either result.
- Write each finding with its file, line, rule, observed behavior and concrete fix. Distinguish a reproduced defect from a question or an unmeasured claim.

## Conditional checks

These items apply only when the repository has the named concern. API, database, image and hexagonal checks require their corresponding capability. Authentication and outbound connections need their own review only where the application uses them; they are not additional capability names.

### Hexagonal code

- Check the dependency directions, ports and shapes in the [context guide](context-template.md). Keep transport, persistence, environment reads and wall-clock access outside domain and application code.
- Declare each port in the package that calls it, never in a central `ports` package: the iface linter reports an interface its own package never uses.
- Give each use case one file, a constructor and `Execute`, with explicit dependencies and plain input and output. The emitting package owns its metrics.

### APIs and authentication

- Start with the contract and keep the generated strict handler signature. Validate examples for each response status, number formats and breaking changes against the consumer contract. Expose the contract hash in liveness.
- Where authorization exists, derive roles from the contract. Test every operation with anonymous input and every unlisted role, expecting the contract's 401 and 403 responses respectively.
- Redact credentials in types, logs and errors. Never log a credential-bearing URI or return internal error chains; map typed failures through the single Problem Details boundary.
- Trust only configured origins and proxies. Reject unknown request fields, and keep the API explorer disabled by default.
- Where cookie sessions exist, use secure, HttpOnly cookies and an explicit SameSite policy. Terminate and revoke sessions on the server, not only by deleting the client's cookie.
- Bound authentication attempts per account and apply the configured abuse controls. Exercise exhaustion and recovery without exposing whether credentials exist.

### Outbound connections

- Guard outbound destinations, schemes and resolved addresses. Check redirects and DNS rebinding so an allowed hostname cannot reach a forbidden address. Keep credentials out of connection diagnostics.
- Set a deadline per call and configure a retry policy for each SDK. Bound attempts and backoff within the operation budget; retries must also fit any transaction budget they consume. Cancel outstanding work when the parent operation ends.

### Databases and persisted state

- Keep network calls outside transactions. Database commands belong to the transaction; remote service calls do not. Commit a money-path change as one database unit.
- Bound retries, provide a parking path, back off and expose permanent failure through liveness. Retry an atomic unit only where its idempotency and remaining budget permit it; do not hold a transaction open during remote calls or backoff.
- Configure statement_timeout, lock_timeout and idle_in_transaction_session_timeout at the pool boundary. Require a deadline per call and a configured retry policy for each SDK; attempts and delays must fit the operation and transaction budgets.
- Test simultaneous duplicate delivery and the partial unique index. Exercise rollback, retry and the result returned to each competing caller.
- For queries over growing tables, use generate_series, ANALYZE and EXPLAIN with representative volume and distributions. Preserve the observed plan and check that the intended index bounds the work; a tiny table's plan is not growth evidence.
- Keep SQL in `db/queries` and generate repository calls with sqlc where configured. Bound multi-row queries and inspect transaction scope and locking along with the query plan.
- Keep applied migrations immutable. Expand first, contract in a later release and avoid renaming in place. Test rollback to the empty starting state and reapplication in an isolated database, including data preservation and recovery.
- Price constraints use CHECK, then NOT VALID and VALIDATE when expanding an existing table. Review the lock budget for the separate validation step.
- Store a version beside every persisted fingerprint. Every finite retention policy has a purge query. Test version compatibility and purge boundaries where those policies change.

### Images

- Review every applicable build and runtime stage, digest pins, installed-tool versions and the profile's allowed sources. Verify the resulting image and scan evidence for the target architecture; a Dockerfile review alone does not prove the built image.

### Service configuration and operations

- Keep configurable values in `configs/` with env tags, defaults, validation and documentation. Review startup rejection of invalid values and the effect of a changed default.
- Use one version per process and log service, env, version and trace identifiers. Keep secrets redacted and give each metric to the package that emits it.
- The highest finite histogram bucket covers the configured ceiling. Budget deadlines from the request boundary to the database. Include queueing, downstream calls, retries and cleanup in that budget.

## Evidence

The delivery record names the revision, input universe, capabilities, tool versions, operating system and architecture. For each claim give the exact command, exit code, relevant stderr and observed result, with a path or link to the output. Name whether the source was the worktree, index, built artifact or remote CI revision; evidence from a different tree is not interchangeable.

List unit and race tests, lint, formatting, scan, coverage and generator checks relevant to the change. Separately state whether mutation, fuzzing, benchmarks and integration ran, which scope they measured, or why they remain unmeasured. Keep those four activities on demand, including in remote execution; no checklist requires arming a local rung.

For mutation, name the package, tested inputs, killed and surviving mutants and the disposition of each survivor. An equivalence claim names the applied input and identical observed result. Do not infer equivalence merely because a test passed or a mutant survived. For fuzzing, record the seed corpus and duration and turn a discovered failure into a deterministic regression test. For benchmarks, record the environment and distributions behind a budget claim.

An unmeasured result is neither a finding nor clean evidence. A check outside its declared subset names its limit, including the file, line and construct on which the verdict depends; unrelated unsupported syntax does not require a verdict. An armed rung names that limit and leaves it to the pipeline. The pipeline cannot report clean while a required check remains unmeasured. A missing capability makes the check inapplicable, with the reason retained in the report.
