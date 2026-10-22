# Rules

golaw separates measurable rules from engineering judgment. The production registry and the four proof manifests of internal/proof identify the checks delivered by M1, M2 and M3; every registered check has a red fixture and a near miss. This page organizes the checks by their stable rule references, which enter every finding's fingerprint and never change, then states what each configuration reader reads and the rewrite that clears a stated limit, and lists the make targets the ci target carries.

## How a check answers

Policy-file checks are tree checks in scan and staged scan. They do not run in agent events or the commit-msg rung. A check applies only where its capability holds, so an absent capability answers NotApplicable. A definite recognized violation is a finding. A configuration the snapshot cannot read is a measurement failure, Unmeasured with its reason. A construct a reader does not read is a stated limit when the verdict depends on it. An unsupported construct matters only when the verdict depends on it. Outside the declared subset is Unmeasured, never a finding. The reason names the file, line and construct. It ends with `outside the declared subset, a stated limit; docs/rules.md names the rewrite`, which points to the reader table of this page. A required Unmeasured check makes scan exit 2. An armed pre-commit rung reports a declared-subset limit and leaves it to the pipeline. A stated limit names the construct, and docs/rules.md names the rewrite that clears it. Missing optional configuration is not a reason to demand that a developer enable a local aid.

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

## Engineering rules by mechanism

Each group names its why and the mechanism that delivers it; a group whose mechanism is prose is a judgment of the review, never a binary check.

- Size and complexity floors: a unit a reader holds whole; goast.file-length, goast.function-length and the three Sonar checks, plus the funlen and gocognit limits of the lint asset through make lint.
- Error identity and wrapping: failures classified by identity, never by text; goast.sentinel-decl, goast.error-text-as-data and goast.init-if-err-nil, while the failures.Prefix discipline is prose of the skill.
- Layer boundaries and Clock injection: domain and use cases free of transport, persistence, environment and the wall clock; the depguard rule of each inner layer under hexagonal (policy.golangci), goast.env-read, goast.bare-context, goast.wall-clock and goast.clock-utc, with the context guide as prose.
- Test isolation and coverage universes: the test target runs with the race detector and -coverpkg=./..., coverage-check holds the floors over the build universe through golaw coverage, goast.goleak holds goroutines; mutation, fuzz, benchmarks and integration are on demand, their scope prose of the evidence.
- Generated tracking and freshness: generated code is versioned and proven fresh by the pipeline; paths.generated-tracked and policy.generated-fresh, the generate-check target running the generators.
- Supply-chain age and records: a version enters seven days after publication and every direct module has a reviewed record; policy.deps-age-config, policy.dependabot-cooldown, policy.dependency-records, policy.image-digest, policy.gotoolchain-local and the vuln and licenses targets.
- Migration safety: policy.squawk-config holds the configuration migrate-lint reads; expand and contract, lock budgets and rollback tests are prose of the checklists.
- API contracts: policy.openapi-assertions holds the Redocly rules and the contract, openapi-breaking runs oasdiff in the pipeline; the strict handler signature and the contract hash in liveness are prose.
- Configuration and observability: configuration enters once at the edge (goast.env-read) and credentials are typed (goast.secret-fields); metric ownership, log identity and histogram ceilings are prose of the skill and the checklists.

## rules/4.1@1

The text family reads every text file of the tree, in scan and in the agent write events, because the public boundary of section 3 holds for documentation as for code.

| Check | Rule | Needs | Why and boundary |
| --- | --- | --- | --- |
| text.dashes | rules/4.1@1 | none | An em dash or en dash in any text file, because the typographic dashes mark generated prose and the standard writes commas, colons and hyphens. |
| text.trace-terms | rules/4.1@1 | none | A commit trailer, an orchestration trace word or an absolute home path in a text file, because the public tree carries no trace of the machine or the process that produced it. |
| text.tool-terms | rules/4.1@1 | none | A private tool name outside the allowed path prefixes, because the policy names the tools (terms.tools) and the narrow scopes where they may appear. |
| text.comment-language | rules/4.1@1 | none | A Go comment in Spanish or Portuguese, judged by function words or two accents outside quotes, because comments are English and only the human channel is localized. |
| text.todo | rules/4.1@1 | none | A task marker in a Go file, because the work is finished or the decision is recorded. |
| text.bare-nolint | rules/4.1@1 | none | A lint suppression without its linter or without a reason, because a bare suppression silences every linter and says nothing. |
| text.nolint-nilnil | rules/4.1@1 | none | A nil, nil return hidden behind a suppression in application code, because the suppression hides the defect the linter found. |
| text.debug-print | rules/4.1@1 | none | A print to standard output outside cmd/ and scripts/, because the composition root owns the process output. |

## rules/4.2@1

| Check | Rule | Needs | Why and boundary |
| --- | --- | --- | --- |
| paths.forbidden | rules/4.2@1 | none | An image or document outside public/, manual/, fixtures/ or testdata/, or a file named like secret material anywhere, because the repository holds code and its evidence, never binaries or credentials. |
| paths.generated-tracked | rules/4.2@1 | none | An ignore file that hides gen/, sqlcgen/, test/mocks or api/generated, because generated code is versioned and proven fresh by the generate-check target of the pipeline. |
| paths.secret-material | rules/4.2@1 | none | A read event whose path, or the file its links lead to, names secret material, because an agent must not load a credential into its context; a source file named like a credential passes. |

## rules/4.3@1

The goast family judges Go files through the parsed syntax tree once the capability go is on. The file and function limits are the lines of the policy; the kit's own laws are 300 and 40 under funlen semantics.

| Check | Rule | Needs | Why and boundary |
| --- | --- | --- | --- |
| goast.file-length | rules/4.3@1 | go | A Go file over the file lines of the policy, because a file a reader cannot hold whole hides its structure. |
| goast.function-length | rules/4.3@1 | go | A function over the function lines or statements of the policy, counted as funlen counts with comments ignored, because the two tools must never disagree in silence. |
| goast.env-read | rules/4.3@1 | go | An environment read outside configs/ and a composition root, because configuration enters once, validated, at the edge. |
| goast.bare-context | rules/4.3@1 | go | A context created outside a composition root, because a bare context drops the deadline and the trace of its caller. |
| goast.wall-clock | rules/4.3@1 | go | The process clock inside the domain or the use cases, because time is a port the tests control. |
| goast.clock-utc | rules/4.3@1 | go | A clock handed to the contexts that is not UTC, because a local clock makes persisted times ambiguous. |
| goast.sql-from-strings | rules/4.3@1 | go | A query formatted or joined from strings when sqlc.yaml exists, because the generated repository is the only SQL boundary of such a repository. |
| goast.raw-error-response | rules/4.3@1 | go,api | A 4xx or 5xx answered by a handler outside the errors package, because the contract's error shape is mapped at one boundary. |
| goast.error-text-as-data | rules/4.3@1 | go | The text of an error stored as data, because errors are classified with errors.Is against sentinels, never by their text. |
| goast.init-if-err-nil | rules/4.3@1 | go | An if with an initializer that tests err == nil, because the happy path hides the failure the initializer produced. |
| goast.sentinel-decl | rules/4.3@1 | go | A package-level errors.New outside errors.go, because a package declares its failure modes in one file. |
| goast.secret-fields | rules/4.3@1 | go | A credential in a bare string field, because a typed secret redacts itself in logs and errors. |
| goast.goleak | rules/4.3@1 | go | A package that starts goroutines without a goleak test, because a leaked goroutine outlives the test that started it. |
| goast.adapter-package-size | rules/4.3@1 | go | An adapter package root holding more Go files than its max_files, because a large adapter mixes concerns the layers keep apart. |
| goast.sonar-empty-body | rules/4.3@1 | go | A function body with no statement and no comment (S1186), because an empty body hides an unfinished decision. |
| goast.sonar-literal-repeat | rules/4.3@1 | go | A literal repeated in one file (S1192), because a repeated literal drifts when one copy changes. |
| goast.sonar-identical-functions | rules/4.3@1 | go | Two functions of one file with the same body (S4144), because a duplicate body is a defect fixed twice. |

## rules/4.5@1

The lint configuration delivers most engineering rules: the required linters, the shadow analysis, the depguard rule of each inner layer, the forbidigo patterns and a ruleguard file for the one rule no linter ships. The checks read the configuration; make lint runs the linters in the pipeline.

| Check | Rule | Needs | Why and boundary |
| --- | --- | --- | --- |
| policy.golangci | rules/4.5@1 | go | A root .golangci.yml missing, a required linter off or disabled, a required linter freed by an exclusion the allowed_exclusions of golaw.yml do not name (a text or source pattern that matches every issue, such as .*, narrows nothing), govet without shadow, an inner layer without a strict depguard rule under hexagonal, or float32 not forbidden, because make lint enforces only what the configuration enables. |
| policy.ruleguard | rules/4.5@1 | go | A gocritic without its ruleguard check, a ruleguard without the rules file it names, or a rules file without the byte cut rule, because no linter golangci-lint ships carries that rule; a go.mod without the dsl stops make lint and is init's proposal, not a finding. |

## rules/4.6@1

The test target runs the suite with the race detector and writes coverage.out; coverage-check holds 80 percent total, per package and per file over the build universe through golaw coverage, with the 20 statement minimum and no exemption. Mutation, fuzz, benchmarks and integration stay on demand.

| Check | Rule | Needs | Why and boundary |
| --- | --- | --- | --- |
| policy.hook-tests | rules/4.6@1 | none | A script directly under scripts/hooks without its .test.sh sibling, because a hook that runs on every commit is code and needs its test; applies only under an enrolled profile, which brings the convention. |

## rules/4.7@1

| Check | Rule | Needs | Why and boundary |
| --- | --- | --- | --- |
| policy.gitleaks-generic | rules/4.7@1 | none | A .gitleaks.toml whose allowlist frees a generated file from the secret scan by path alone, a file of the tree whose header marks it generated or one of the common generated paths, because a credential written into generated code is as live as any other; an absent file and a conditional allowlist are no finding, and an [extend] path or url is a stated limit. |
| shell.secret-tokens | rules/4.7@1 | none | A command line that names secret material, judged by name and never read, because the name list that denies a Read denies the command that prints the file. |
| shell.credential-print | rules/4.7@1 | none | A command that prints a credential into the context (printenv, env with a credential name, gh auth token, security find-generic-password -w, aws configure get, op read, vault read, kubectl get secret), because the context of an agent is a log. |
| shell.push | rules/4.7@1 | none | A git push, denied when the policy says so (deny_push) and reported as a note otherwise, because the core leaves the push to the person's judgment. |
| shell.pr | rules/4.7@1 | none | A gh pr create or gh repo create, denied when the policy says so (deny_pr) and reported as a note otherwise, because a pull request is the person's act. |

## rules/4.8@1

A version enters seven days after publication, every direct module has a decision record, generated trees are proven fresh, the toolchain is the one the machine holds, and an image base is pinned by digest. An external local replace names its directive and leaves dependency measurement Unmeasured.

| Check | Rule | Needs | Why and boundary |
| --- | --- | --- | --- |
| policy.gotoolchain-local | rules/4.8@1 | go | A Makefile that does not export GOTOOLCHAIN=local (a bare export line or .EXPORT_ALL_VARIABLES exports a plain assignment no unexport line names), a golang build stage of a Dockerfile that sets another value under image, or an Actions step that runs go after actions/setup-go without restoring GOTOOLCHAIN=auto, because make and the image run the toolchain the machine holds and go.mod decides it in CI. |
| policy.generated-fresh | rules/4.8@1 | go | A Makefile without a generate-check target, because the pipeline proves generated code fresh by running the generators and diffing; running them belongs to the target, not to the check. |
| policy.dependency-records | rules/4.8@1 | go | A direct require of go.mod or tools/go.mod without a record whose front matter names the module under the decisions directory, because prose cannot authorize a dependency nobody reviewed; a replace inside the repository is followed. |
| policy.dependabot-cooldown | rules/4.8@1 | none | A gomod or github-actions update of .github/dependabot.yml or .github/dependabot.yaml, other than one with open-pull-requests-limit 0, which serves security updates alone, whose cooldown is under seven days, under thirty for a major version of a Go module, or carries an exclude list or an include list narrower than *, because the cooldown is the update age of the supply chain; an absent file is NotApplicable, and both names at once are a stated limit. |
| policy.deps-age-config | rules/4.8@1 | go | A deps-age target that does not run the kit's depsage with -days 7 or more, through its recipe or a target it delegates to, because the pipeline that runs make ci is the only place the age floor holds. |
| policy.image-digest | rules/4.8@1 | image | A FROM, a COPY --from or the from of a RUN --mount of a Dockerfile outside the fixtures whose external image carries no sha256 digest and matches no pattern of allowed_tags, because a tag moves and a digest does not; scratch, an earlier stage and a stage index are not images. |

## rules/4.9@1

| Check | Rule | Needs | Why and boundary |
| --- | --- | --- | --- |
| policy.squawk-config | rules/4.9@1 | db | A .squawk.toml missing, with a pg_version that is not numeric, assume_in_transaction off, an excluded rule other than prefer-identity, or an excluded_paths entry that frees a migration of db/migrations/, because migrate-lint enforces only what its configuration keeps. |

## rules/4.10@1

| Check | Rule | Needs | Why and boundary |
| --- | --- | --- | --- |
| policy.openapi-assertions | rules/4.10@1 | api | A redocly.yaml missing or without the error-level number-double, response-example and example-validation rules, or a contract it names whose number schemas lack format double or whose responses lack an example, because consumers hold the contract to those assertions; a contract not written yet is skipped, and a redocly.yml, .redocly.yaml or .redocly.yml in place of an absent redocly.yaml is a stated limit whose reason names the rename. |

## rules/4.12@1

The commits family applies rule `rules/4.12@1` to the message file and staged changes at the optional commit-msg rung. The shell family retains the same rule for supported agent commands.

| Check | Rule | Needs | Why and boundary |
| --- | --- | --- | --- |
| policy.codeowners-foreign | rules/4.12@1 | none | A staged change to a path the CODEOWNERS of the base assigns to another owner, because a staged edit cannot authorize itself; applies only under an enrolled profile, and only to staged changes. |
| shell.no-verify | rules/4.12@1 | none | A command that skips the git hooks with --no-verify, -n on commit or a hooks path override, because an armed rung is the developer's choice and a bypass is not. |
| shell.trailers | rules/4.12@1 | none | A trailer in a commit made from the command line, as a phrase or a flag, because the recorded message carries no trailer. |
| shell.dashes | rules/4.12@1 | none | An em dash or en dash in a commit message given on the command line, because the commit law is the text law. |
| shell.body-width | rules/4.12@1 | none | A body line over 100 characters in a message given on the command line, because the body is read in a terminal; the subject is the commit rung's business. |
| shell.shape | rules/4.12@1 | none | A commit whose type is not feat that stages more than 40 new files or 400 new lines through the same command line, because a commit that is not a feature carries one theme. |
| commit.conventional | rules/4.12@1 | none | A subject without a recognized conventional type, an optional scope and breaking-change marker, then a colon, a space and a description starting with a lowercase ASCII letter, because the type drives the changelog and the review. |
| commit.no-trailers | rules/4.12@1 | none | A trailer of any kind in the recorded message, because the public tree carries no trailer. |
| commit.no-dashes | rules/4.12@1 | none | An em dash or en dash in the recorded subject or body, because the text law holds for the history. |
| commit.body-width | rules/4.12@1 | none | A recorded body line longer than 100 Unicode characters, because the body is read in a terminal. |
| commit.shape | rules/4.12@1 | none | A commit other than feat adding more than 40 new files or 400 new lines, because a commit that is not a feature carries one theme. |

The conventional types are `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `build`, `ci`, `perf`, `style` and `revert`. Recognized Git-generated merge, revert and reapply subjects are exempt from commit.conventional and commit.shape. The other message checks still apply. Autosquash prefixes `fixup! `, `squash! ` and `amend! ` are removed before the underlying subject and type are judged.

The checks judge the message after Git cleanup. The rung reads `commit.cleanup` and the configured comment character or string. Default cleanup strips comments when an editor was used and retains them when Git disables the editor. Strip cleanup removes comments and excess whitespace; whitespace and scissors cleanup retain comments; verbatim retains the remaining bytes. The scissors separator and the text below it are excluded. Under strip cleanup with an ambiguous automatic comment choice, possible comment lines remain unjudged rather than causing a false denial.

The body begins after the first nonblank subject line. A body line of exactly 100 characters passes; 101 fails. A trailing carriage return does not add a character. The shape boundaries are inclusive: 40 new files and 400 new lines pass. Modified, renamed and deleted files are not new files. A `feat` commit is exempt from both shape limits, and paths brought by a merge are excluded before the changes reach commit.shape.

commit.author-noreply is deferred: M2 never infers that a repository is public. M2 does not add a remote commit-range message gate. policy.hook-tests and policy.codeowners-foreign require an enrolled profile. Without staged changes, policy.codeowners-foreign is NotApplicable.

## rules/4.13@1

The ladder of section 7: setup alone arms the optional git rungs, each switch of `ladder` in golaw.yml stays freely disabled without a finding, the push rung runs only the vulnerability scanner, the optional staged tests stay off by default, and the heavy suites stay on demand. The pre-commit rung names three steps it leaves to the pipeline and runs no tool for them: migrate-lint when a `.sql` under a directory named `migrations` is staged under db, openapi-breaking when a file named `openapi.yaml`, `openapi.yml` or `openapi.json` is staged under api, and generate-check when the index tracks `.mockery.yaml` and a non-test `.go` file or that file is staged, tracks `sqlc.yaml` and a `.sql` file or that file is staged, or tracks a file under `api/codegen/` and such a file or the contract is staged. The push rung runs govulncheck only.

| Check | Rule | Needs | Why and boundary |
| --- | --- | --- | --- |
| policy.makefile | rules/4.13@1 | go | A Makefile absent, or a Makefile with its includes that lacks a required target under the capabilities in force, names a required include the tree does not hold, or gives a required target two recipes, because the pipeline runs make ci and its targets must exist; an expression that could define a missing target is a stated limit, as is a GNUmakefile or makefile in place of an absent Makefile, whose reason names the rename. |
| policy.ci-target | rules/4.13@1 | go | A ci target whose prerequisites lack a required target that no scheduled or manually dispatched Actions job runs as make target, or a lefthook.yml whose pre-commit, commit-msg or pre-push runs a check heavier than the binary (make target, go test, go vet, go build, golangci-lint run, gremlins unleash), because a check leaves the ci target only by moving to on demand and an armed rung finishes in seconds. |

An absent Makefile is named by policy.makefile alone. A missing required target is also named by the check whose target it is (policy.ci-target, policy.generated-fresh, policy.deps-age-config). A missing lefthook.yml is not a finding. Makefile includes resolve from the repository root. An unused Makefile variable does not widen the reader's scope.

The required targets are data, `policyfiles.RequiredTargets`, read alike by the two checks, the golaw.mk asset, the CI template and the acceptance rows. scan remains the golaw scan; image-scan names the image scan. docs-check is the api row, because the kit ships no generator of a configuration page and the bundled contract is the page the pipeline keeps fresh. hooks-test is profile data, not a core required target. cli and hexagonal add no make target. Without go the answer is the capability rows alone.

| Capabilities | Targets the ci target carries, in order |
| --- | --- |
| none | scan |
| image | scan dockerfile-lint image-pins-check image image-scan |
| go | mod deps-age fmt vet generate-check test coverage-check build lint deadcode vuln licenses secrets scan |
| go,cli | mod deps-age fmt vet generate-check test coverage-check build lint deadcode vuln licenses secrets scan |
| go,api | mod deps-age fmt vet generate-check test coverage-check build lint deadcode openapi-breaking openapi-examples-check vuln licenses docs-check secrets scan |
| go,db | mod deps-age fmt vet generate-check test coverage-check build lint deadcode migrate-lint vuln licenses secrets scan |
| go,image | mod deps-age fmt vet generate-check test coverage-check build lint deadcode vuln licenses secrets scan dockerfile-lint image-pins-check image image-scan |
| go,hexagonal | mod deps-age fmt vet generate-check test coverage-check build lint deadcode vuln licenses secrets scan |
| go,api,cli,db,hexagonal,image | mod deps-age fmt vet generate-check test coverage-check build lint deadcode migrate-lint openapi-breaking openapi-examples-check vuln licenses docs-check secrets scan dockerfile-lint image-pins-check image image-scan |

Two targets run the kit's own tools: `deps-age` runs `tools/depsage` with `-days 7`, and `vuln` runs `tools/vulngate` with `-scan -waivers golaw.yml`, which runs the pinned govulncheck itself and judges its stream against the committed waivers. vulngate exits 0 when clean, 1 for a reachable advisory without a valid waiver and 2 when the scanner cannot start or its stream is incomplete; a failed or incomplete remote vulnerability measurement is not clean evidence. An adopter's golaw.mk selects both tools at the kit pin, `go run github.com/cristianargotti/golaw/tools/depsage@v<pin> -days 7`, each such target preceded by its target-specific `<target>: export GOTOOLCHAIN := auto`, because a tool's go line may be newer than the repository's while `export GOTOOLCHAIN=local` keeps the module's own builds on the toolchain the machine holds; a make export keeps the recipe one plain command under cmd.exe. The kit runs its local source, `go run ./tools/depsage -days 7` and `go run ./tools/vulngate -scan -waivers golaw.yml`, and its scheduled lane runs `make vuln`.

## Reader subsets and stated limits

Each reader of `internal/checks/policyfiles/` reads one format through the subset below; a construct outside it that a verdict depends on is a stated limit whose reason ends with the sentinel of this page, and the last column names the rewrite within the subset that clears it, so the person or the agent fixes it without help. A stated limit fails the only gate, scan exits 2, and no exception covers it.

| Reader | File | Declared subset | Stated limit | How to resolve |
| --- | --- | --- | --- | --- |
| readGolangci | lint_golangci.go | Version 2 configuration with effective exclusions, required and disabled linters, shadow analysis, capability-dependent boundary rules, forbidden patterns and ruleguard linkage. | a version other than "2" is outside the subset; relevant unsupported shapes inherit the shared YAML limits. | run golangci-lint migrate for a version 1 file, and keep one YAML configuration at the root. |
| readMakefile | build_makefile.go | Explicit rules, merged prerequisites and recipes, supported assignments and exports, conditional blocks read without evaluating them, define blocks as one assignment, target-specific assignments, literal includes resolved from the repository root, missing required and optional includes, and snapshot read errors. | a computed include name and an include cycle are outside the subset; a double-colon rule is recorded with its targets and is a limit, named as double-colon rule <target>::, only for a check that needs one of them; a rule, include or export inside a conditional block is recorded with its opener, and a check that needs a target or variable only such a block or a top-level $(eval) or $(call) line could define states a limit naming it; exported computed values are recorded raw for the check that reads them, and no command is evaluated; when ci misses a required target, a recipe line of a target ci reaches that could start make (make with a flag or a computed target, a shell, an interpreter, a command wrapper, a script path, a shell operator or $(shell)) is an opaque recipe, whose reason names the missing targets, while a line whose command is another literal program, such as go build, is skipped. | name the include literally, break the cycle, use a single-colon rule, and define the required target or the export outside the conditional block, as an explicit rule where an $(eval) line generated it; for an opaque recipe, add the missing target to ci, and write the delegation as a prerequisite of ci or as make <target> on its own line. |
| readLefthook | build_lefthook.go | Every command and script of every hook in file order, decoded through the shared YAML reader; checks recognize the heavier commands make ci carries. | a second document is outside the subset, as is a nonmapping root; relevant indirect configuration follows the producer's stated limits and missing optional configuration is not a finding. | write each command as one run string in lefthook.yml itself. |
| readActions | build_actions.go | YAML files in the Actions configuration directory, through the snapshot in path order, including triggers, jobs, steps and environment fields; scheduled or manually dispatched jobs supply on-demand target evidence. | Only files directly under that directory are read; expressions and shell programs are not executed, and unsupported relevant forms remain stated limits. | keep the file directly under .github/workflows/ and write the make target literally on its own line. |
| readModule | deps_gomod.go | The module path, direct requires from every block and replaces through parseModule and modfile; exact record front-matter matching, the application module and tools/go.mod, and in-repository replacement traversal. | The boundary is the modfile grammar, with every require and replace block included; syntax errors remain measurement failures and an external local replace names its directive and leaves dependency measurement Unmeasured. | replace the module with a path inside the repository, or drop the replace once the module is published. |
| readDockerfile | deps_dockerfile.go | Ordered stages, aliases, image references, ARG defaults and ENV assignments of every file isDockerfile selects, and the --from of COPY and the from of a RUN --mount; policy.image-digest judges the digest of each external reference. | A FROM image named by an ARG without a default, a parameter expression such as ${BASE:-alpine}, and an unsupported heredoc form or parser directive are outside the subset; for policy.image-digest, an expansion in a --from or a mount from, and a bare name no earlier stage carries, which may be a build context, are outside it too; the value of --platform is never judged. | give the ARG a literal default, write the reference literally, name an earlier stage or write the image with its registry or tag, and use a plain heredoc. |
| readOwners | deps_codeowners.go | The grammar of bootstrap.AllowedPaths, including ordered ownership patterns; the check reads staged paths against the protected base. | a negation or a bracket is outside the subset; the check is NotApplicable without staged changes. | write the pattern without a negation or a bracket, one path per rule. |
| readDependabot | deps_dependabot.go | Update entries and cooldown settings through decodeYAML, with the version, integer day values and include/exclude forms of policy.dependabot-cooldown. | a second document is outside the subset, as is a nonmapping root; unsupported relevant update shapes are stated limits, while an absent optional file is not a finding. | keep one document with a mapping root and integer cooldown days. |
| decodeYAML | yaml.go | One mapping document, aliases followed, own keys first and then merged anchors in order; an empty document gives an empty mapping. | a root that is not a mapping or a second document is outside the subset; syntax errors are measurement errors. | keep one document whose root is a mapping. |
| decodeTOML | formats.go | Whole-file TOML decoding for the secret-scanner and migration configuration; consumers inspect their declared keys. | a syntax error naming the file and the line is a measurement error; no narrower TOML syntax subset is invented. | fix the syntax at the line named. |
| decodeYAML (Redocly) | yaml.go | policy.openapi-assertions consumes this shared reader in deps_redocly.go for recommended or recommended-strict inheritance, literal local API roots, effective global and per-API rules, number format double and response examples, and the exceptions of .redocly.lint-ignore.yaml; present OpenAPI sources and local references follow that check. | a second document is outside the subset; unknown inheritance, plugins or relevant assertion expressions are named limits, as are remote references, unsupported document dialects and an ignore entry outside its subset. | extend recommended or recommended-strict, write the rule in the subject, where and assertions forms the template shows, and keep references local. |

## Repository laws

Files contain at most 300 lines. Functions contain at most 40 lines under funlen semantics, with comments ignored. Tests retain the file limit and lose the function limit.

Coverage is at least 80 percent total, per package, and per file, with the 20 statement minimum and no kit exemptions.

Comments explain why in English. Documentation, findings, and commits are English. Human-channel strings use the configured language. Conventional commits have no trailers.

Local checks are optional aids. Remote CI or the platform pipeline is the only hard gate.

The prose mapping above connects the retained inventory to the skill, this page and the delivered builder and reviewer checklists.
