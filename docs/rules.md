# Rules

golaw separates measurable rules from engineering judgment. The production registry and internal/proof/manifest.yml identify the checks delivered by M1 and M2. Every registered check has a red fixture and a near miss.

## Prose mapping

The inventory rows below identify retained engineering judgment and its delivered text. They are prose labels, not check IDs, and do not change published `rules/4.x@1` references or finding fingerprints. Apply each item only where the repository has the concern it describes; judgment stays prose rather than becoming a runtime finding.

| Inventory row | Delivered location |
| --- | --- |
| 4.1: one use case per file | agents/SKILL.md |
| 4.1: pull request size and topic | agents/SKILL.md |
| 4.2: truthful why comments | agents/SKILL.md |
| 4.3: HTTP error identity | agents/SKILL.md |
| 4.4: third-party boundary dependencies | docs/rules.md |
| 4.4: context shapes | docs/rules.md |
| 4.4: metric ownership | agents/SKILL.md |
| 4.4: transactions and retries | docs/checklists.md |
| 4.6: wall-clock budgets | docs/checklists.md |
| 4.6: mutant equivalence | docs/checklists.md |
| 4.6: growing-table query plans | docs/checklists.md |
| 4.6: falsifiable checks and harness results | docs/checklists.md |
| 4.6: amd64 delivery evidence | docs/checklists.md |
| 4.6: workstation concurrency | docs/checklists.md |
| 4.9: price constraints | docs/checklists.md |
| 4.9: fingerprint versions | docs/checklists.md |
| 4.9: finite retention | docs/checklists.md |
| 4.9: pool and call budgets | docs/checklists.md |
| 4.9: concurrent idempotency | docs/checklists.md |
| 4.10: contract identity in liveness | docs/checklists.md |
| 4.11: process and log identity | docs/checklists.md |
| 4.11: histogram ceilings | docs/checklists.md |
| 4.11: end-to-end deadlines | docs/checklists.md |
| 4.12: destination conventions | docs/checklists.md |
| 4.12: squash trailer risk | docs/checklists.md |
| 4.15: strongest documented practice | agents/SKILL.md |
| 4.15: reuse ladder | agents/SKILL.md |
| 4.15: managed-object justification | docs/checklists.md |
| 4.15: measurement falsifiability | docs/checklists.md |
| 4.15: reproducible claims | agents/SKILL.md |
| 4.15: agreement between tools | docs/checklists.md |
| 4.15: recurring findings and lessons | docs/checklists.md |
| 4.15: verification and review | docs/checklists.md |
| 4.15: builder and reviewer checklists | docs/rules.md |

A third-party dependency in domain or application needs a decision record and an explicit allowed boundary. The record explains why repository code and the standard library cannot meet the need, the alternatives and the reversal condition. The allowed import boundary prevents a convenience dependency from silently introducing transport or infrastructure into the inner layers; a record alone does not widen the policy.

Context shapes use plain application structs, repository row mapping, generated handler signatures and validated configuration. These shapes keep domain and use cases independent of transport, persistence and environment parsing. The [context guide](context-template.md) gives the directories, ports, construction order and tests without introducing an application framework or scaffolding application source.

The builder and reviewer use docs/checklists.md. The [builder checklist](checklists.md#builder) orders decisions, implementation and delivery evidence; the [reviewer checklist](checklists.md#reviewer) judges that evidence against boundaries and failure modes. They stay distinct so completing implementation steps does not substitute for judging the result. A clean review needs no invented finding.

The [canonical skill](../agents/SKILL.md) also retains failure wrapping with failures.Prefix, errors.Is against sentinels, the Clock port, integration and migration conventions, scan guidance and permission before a push or pull request. See the [security guide](security.md) for assurance and recovery, and [agent contracts](agent-contracts.md) for protocol and live evidence.

## Go source checks

The goast family judges Go files through the parsed syntax tree, after the capability go is on. Each check has a red fixture and a near miss in internal/proof/manifest.yml.

| Check | What it refuses |
| --- | --- |
| goast.file-length | A Go file over the file lines of the policy |
| goast.function-length | A function over the function lines or statements of the policy, counted as funlen counts |
| goast.env-read | An environment read outside configs/ and a composition root |
| goast.bare-context | A context created outside a composition root |
| goast.wall-clock | The process clock inside the domain or the use cases |
| goast.clock-utc | A clock handed to the contexts that is not UTC |
| goast.sql-from-strings | A query formatted or joined from strings when sqlc.yaml exists |
| goast.raw-error-response | A 4xx or 5xx answered by a handler outside the errors package |
| goast.error-text-as-data | The text of an error stored as data |
| goast.init-if-err-nil | An if with an initializer that tests err == nil |
| goast.sentinel-decl | A package-level errors.New outside errors.go |
| goast.secret-fields | A credential in a bare string field |
| goast.goleak | A package that starts goroutines without a goleak test |
| goast.adapter-package-size | An adapter package root holding more Go files than its max_files |
| goast.sonar-empty-body | A function body with no statement and no comment (S1186) |
| goast.sonar-literal-repeat | A literal repeated in one file (S1192) |
| goast.sonar-identical-functions | Two functions of one file with the same body (S4144) |

## Commit checks

The commits family applies rule `rules/4.12@1` to the message file and staged changes at the optional commit-msg rung. The shell family retains the same rule for supported agent commands.

| Check | What it refuses |
| --- | --- |
| commit.conventional | A subject without a recognized conventional type, an optional scope and breaking-change marker, then a colon, a space and a description starting with a lowercase ASCII letter |
| commit.no-trailers | A trailer of any kind in the recorded message |
| commit.no-dashes | An em dash or en dash in the recorded subject or body |
| commit.body-width | A recorded body line longer than 100 Unicode characters |
| commit.shape | A commit other than feat adding more than 40 new files or 400 new lines |

The conventional types are `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `build`, `ci`, `perf`, `style` and `revert`. Recognized Git-generated merge, revert and reapply subjects are exempt from commit.conventional and commit.shape. The other message checks still apply. Autosquash prefixes `fixup! `, `squash! ` and `amend! ` are removed before the underlying subject and type are judged.

The checks judge the message after Git cleanup. The rung reads `commit.cleanup` and the configured comment character or string. Default cleanup strips comments when an editor was used and retains them when Git disables the editor. Strip cleanup removes comments and excess whitespace; whitespace and scissors cleanup retain comments; verbatim retains the remaining bytes. The scissors separator and the text below it are excluded. Under strip cleanup with an ambiguous automatic comment choice, possible comment lines remain unjudged rather than causing a false denial.

The body begins after the first nonblank subject line. A body line of exactly 100 characters passes; 101 fails. A trailing carriage return does not add a character. The shape boundaries are inclusive: 40 new files and 400 new lines pass. Modified, renamed and deleted files are not new files. A `feat` commit is exempt from both shape limits, and paths brought by a merge are excluded before the changes reach commit.shape.

commit.author-noreply is deferred: M2 never infers that a repository is public.

M2 does not add a remote commit-range message gate.

Migration, API and generation rung tools arrive with the M3 templates.

## Repository laws

Files contain at most 300 lines. Functions contain at most 40 lines under funlen semantics, with comments ignored. Tests retain the file limit and lose the function limit.

Coverage is at least 80 percent total, per package, and per file, with the 20 statement minimum and no kit exemptions.

Comments explain why in English. Documentation, findings, and commits are English. Human-channel strings use the configured language. Conventional commits have no trailers.

Local checks are optional aids. Remote CI or the platform pipeline is the only hard gate.

The prose mapping above connects the retained inventory to the skill, this page and the delivered builder and reviewer checklists.
