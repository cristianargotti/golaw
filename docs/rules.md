# Rules

golaw separates measurable rules from engineering judgment. The production registry and internal/proof/manifest.yml identify the checks delivered by M1 and M2. Every registered check has a red fixture and a near miss.

## Prose mapping

| Rule | Delivered location |
| --- | --- |
| Reuse ladder and dependency justification | agents/SKILL.md |
| failures.Prefix, sentinels, and errors.Is | agents/SKILL.md |
| One use case per file | agents/SKILL.md |
| Pull request size and one topic | agents/SKILL.md |
| Permission before push or pull request | agents/SKILL.md |
| Strongest documented practice | agents/SKILL.md |
| Clock port and metric ownership | agents/SKILL.md |
| Integration conventions and migration recovery | agents/SKILL.md |
| Test-first evidence and unmeasured results | agents/SKILL.md |
| Scan before finishing | agents/SKILL.md |
| Assurance, secret limits, and recovery | docs/security.md |
| Agent protocol and live evidence | docs/agent-contracts.md |

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

The complete retained-rule mapping and builder and reviewer checklists belong to M3.
