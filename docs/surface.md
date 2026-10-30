# The non-Go surface

golaw judges the files of a Go repository that are not Go: JavaScript, TypeScript, JSX and TSX, the scripts and styles inside HTML, CSS, Python scripts and tests, Dockerfiles, CloudFormation templates, shell scripts and plain YAML. A full Python or TypeScript project, a repository whose root holds a package.json with a build script or a pyproject.toml with a build backend, is outside this surface: its files get the same assets, and golaw doctor prints that the project shape is outside it.

Every check on the developer's machine is optional. The pipeline or remote CI is the only gate. golaw never calls a Sonar server, a scanner or its API, never reads a token for it and never imports its results: parity comes from local linters whose rules map to the Sonar way rules, and a false denial or a false finding is a defect.

## Languages

The languages of a repository are computed from its snapshot on every scan, rung and hook, from the tracked and untracked files, and never stored in golaw.yml: a language appears the moment its first file does.

| Language       | A file is this language when                                                                                                                      |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------- |
| javascript     | its extension is .js, .mjs or .cjs, or it has no extension and a node shebang                                                                     |
| typescript     | its extension is .ts, .mts or .cts; a .d.ts declaration file is judged by name rules only                                                         |
| jsx, tsx       | its extension is .jsx or .tsx                                                                                                                     |
| html           | its extension is .html or .htm; a script body without src and with no type or a JavaScript type is a javascript region, a style body a css region |
| css            | its extension is .css, and every css region of html                                                                                               |
| python         | its extension is .py, or it has no extension and a python shebang                                                                                 |
| dockerfile     | its name is Dockerfile, Dockerfile.*, *.Dockerfile or Containerfile                                                                               |
| cloudformation | a .yml, .yaml, .json or .template file whose top level holds AWSTemplateFormatVersion, or a Resources map with a Type that starts with AWS::      |
| shell          | its extension is .sh or .bash, or it has no extension and a sh, bash, dash or zsh shebang                                                         |
| yaml           | a .yml or .yaml file that is not cloudformation                                                                                                   |

A minified file, a .min. segment in the name or an average line of at least 200 characters, is judged by name rules only, as the remote analyzer skips it. A spliced script, one that holds a Go template action, is judged by the native checks and reported unmeasured by the linters with its line named. A generated file keeps its class: a generated header in any comment syntax, or a path under gen/, sqlcgen/, testdata/ or test/mocks/.

## The analyzer scope

The analyzer scope says which paths the remote analyzer reads as main code, which as test code, and which lines count for coverage. It describes the pipeline and never relaxes a core law: the native checks and every core check apply to the whole tracked tree whatever the scope says. It governs three things only: the files golaw lint reads, whether a test file meets the main-code rules there, and the coverage universe of golaw coverage --new-code.

Under surface.analyzer.from: auto, the default, the first source present wins:

1. the analyzer.yml of the pinned profile bundle, because a platform pipeline may delete the repository's own sonar-project.properties before it scans;
2. sonar-project.properties at the repository root, through a declared subset of Java properties: the keys sonar.sources, sonar.tests, sonar.inclusions, sonar.exclusions, sonar.test.inclusions, sonar.test.exclusions, sonar.coverage.exclusions, sonar.cpd.exclusions and sonar.go.coverage.reportPaths, with key=value and key: value lines, # and ! comments, a trailing backslash continuation and comma separated lists; a ${...} placeholder or a \u escape in a read key is unmeasured with the key named;
3. the lists of surface.analyzer in golaw.yml;
4. none: the whole tracked tree is main code and no file is test code, as a scanner reads a repository that sets no sonar.sources.

from: profile, from: properties and from: policy pin one source and make its absence an error that names it. Patterns follow the Sonar glob, **, * and ? over slash paths relative to the root and case-sensitive; a pattern golaw cannot parse is unmeasured. golaw config explain prints the scope in force, its source and each list. The public kit reads only generic sources: a company's pipeline exclusions live in its private profile, never in this kit (see docs/profiles.md).

```yaml
surface:
  rules: strict
  analyzer:
    from: auto
    sources: []
    tests: []
    test_inclusions: []
    exclusions: []
    coverage_exclusions: []
    coverage_report: profile.cov
  scripts:
    globals: []
ladder:
  {
    pre_commit: true,
    commit_msg: true,
    pre_push: true,
    test_staged: false,
    surface_lint: true,
    hook_feedback: true,
  }
```

surface.rules is a floor: parity tightens to strict and the reverse is refused. surface.analyzer and surface.scripts describe the pipeline and sit outside the floors. ladder.surface_lint and ladder.hook_feedback are local aids, and each switches off without a finding.

## Rule sets

parity holds the local rules whose Sonar way counterpart is exact or close: a repository green under parity is expected green under Sonar way for every mapped key, and the unmapped keys are the gaps below. strict is parity plus the strict market configuration of each language. golaw init --apply writes strict; when the strict set already finds something in the files the repository holds, init prints the count per tool and rule and names golaw init --surface-rules parity as the reviewed starting floor, raised to strict later through a reviewed diff.

The configurations are kit assets embedded in the binary and written into the tool directory, never into the repository. golaw lint passes them explicitly and disables the repository's own configuration lookup (ESLint --config with the kit configuration, which flat ESLint reads alone, ruff --config with the kit file, which replaces any file ruff would discover, shellcheck --norc, hadolint --config), so a repository configuration neither weakens the set nor runs code. A repository that needs a local option names it in golaw.yml: surface.scripts.globals lists the names a server-side splice gives a script.

A suppression that names its rules and gives a reason is allowed, and a bare one is denied at the agent's write by surface.suppression. golaw lint reports each finding ESLint suppressed as a note, because the pipeline's analyzer may not read the suppression; ruff reports no finding a noqa comment suppressed, so a noqa with its code hides the finding from golaw lint alone, and the pipeline may still report it.

## The rule map

Match exact means the local rule implements the Sonar key or the measured counts agree; close means the same intent with another detection; only exact and close rows claim parity. Where: H is the hook on the file just written, N a native golaw check, L the lint target and CI. Set is the lowest rule set that enables the rule.

| Sonar key | Tool | Local rule | Where | Match | Set |
| --------- | ---- | ---------- | ----- | ----- | --- |
| cloudformation:S6249 | golaw | cfn.bucket-https-only | N, L | close | parity |
| cloudformation:S6252 | checkov | CKV_AWS_21 | L | close | parity |
| cloudformation:S6258 | checkov | CKV_AWS_18 | L | close | parity |
| cloudformation:S6258 | checkov | CKV_AWS_91 | L | close | parity |
| cloudformation:S6265 | checkov | CKV_AWS_20 | L | close | parity |
| cloudformation:S6265 | checkov | CKV_AWS_57 | L | close | parity |
| cloudformation:S6281 | checkov | CKV_AWS_53 | L | close | parity |
| cloudformation:S6281 | checkov | CKV_AWS_54 | L | close | parity |
| cloudformation:S6281 | checkov | CKV_AWS_55 | L | close | parity |
| cloudformation:S6281 | checkov | CKV_AWS_56 | L | close | parity |
| css:S4647 | stylelint | color-no-invalid-hex | L | close | parity |
| css:S4648 | stylelint | font-family-no-duplicate-names | L | close | parity |
| css:S4649 | stylelint | font-family-no-missing-generic-family-keyword | L | close | parity |
| css:S4654 | stylelint | property-no-unknown | L | close | parity |
| css:S4656 | stylelint | declaration-block-no-duplicate-properties | L | close | parity |
| css:S4657 | stylelint | declaration-block-no-shorthand-property-overrides | L | close | parity |
| css:S4658 | stylelint | block-no-empty | L | close | parity |
| css:S4659 | stylelint | selector-pseudo-class-no-unknown | L | close | parity |
| css:S4660 | stylelint | selector-pseudo-element-no-unknown | L | close | parity |
| css:S4662 | stylelint | at-rule-no-unknown | L | close | parity |
| css:S4663 | stylelint | comment-no-empty | L | close | parity |
| css:S4664 | stylelint | no-descending-specificity | L | close | parity |
| css:S4666 | stylelint | no-duplicate-selectors | L | exact | parity |
| css:S4667 | stylelint | no-empty-source | L | close | parity |
| css:S4668 | stylelint | unit-no-unknown | L | close | parity |
| css:S5362 | stylelint | function-calc-no-unspaced-operator | L | close | parity |
| docker:S6471 | golaw | docker.final-user | H, N | close | parity |
| docker:S6471 | hadolint | DL3002 | H, L | close | parity |
| docker:S6472 | golaw | surface.hardcoded-credential | H, N, L | close | parity |
| docker:S6500 | hadolint | DL3015 | H, L | close | parity |
| docker:S8431 | golaw | docker.tag-and-digest | H, N, L | exact | parity |
| javascript:S1128 | eslint | sonarjs/unused-import | L | exact | parity |
| javascript:S1135 | eslint | sonarjs/todo-tag | L | exact | parity |
| javascript:S1186 | eslint | @typescript-eslint/no-empty-function | L | exact | parity |
| javascript:S1313 | eslint | sonarjs/no-hardcoded-ip | L | exact | parity |
| javascript:S1481 | eslint | sonarjs/no-unused-vars | L | exact | parity |
| javascript:S1523 | eslint | sonarjs/code-eval | L | exact | parity |
| javascript:S1788 | eslint | @typescript-eslint/default-param-last | L | exact | parity |
| javascript:S1854 | eslint | sonarjs/no-dead-store | L | exact | parity |
| javascript:S1871 | eslint | sonarjs/no-duplicated-branches | L | exact | parity |
| javascript:S1874 | eslint | sonarjs/deprecation | L | exact | parity |
| javascript:S2004 | eslint | sonarjs/no-nested-functions | L | exact | parity |
| javascript:S2068 | eslint | sonarjs/no-hardcoded-passwords | L | exact | parity |
| javascript:S2245 | eslint | sonarjs/pseudo-random | L | exact | parity |
| javascript:S2486 | eslint | sonarjs/no-ignored-exceptions | L | exact | parity |
| javascript:S2589 | eslint | sonarjs/no-gratuitous-expressions | L | exact | parity |
| javascript:S2681 | eslint | sonarjs/no-unenclosed-multiline-block | L | exact | parity |
| javascript:S3358 | eslint | sonarjs/no-nested-conditional | L | exact | parity |
| javascript:S3776 | eslint | sonarjs/cognitive-complexity | L | exact | parity |
| javascript:S3800 | eslint | sonarjs/function-return-type | L | exact | parity |
| javascript:S4144 | eslint | sonarjs/no-identical-functions | L | exact | parity |
| javascript:S4624 | eslint | sonarjs/no-nested-template-literals | L | exact | parity |
| javascript:S5725 | eslint | sonarjs/disabled-resource-integrity | L | exact | parity |
| javascript:S5843 | eslint | sonarjs/regex-complexity | L | exact | parity |
| javascript:S5852 | eslint | sonarjs/slow-regex | L | exact | parity |
| javascript:S6353 | eslint | sonarjs/concise-regex | L | exact | parity |
| javascript:S6479 | eslint | react/no-array-index-key | L | exact | parity |
| javascript:S6551 | eslint | @typescript-eslint/no-base-to-string | L | exact | parity |
| javascript:S6759 | eslint | sonarjs/prefer-read-only-props | L | exact | parity |
| javascript:S6819 | eslint | jsx-a11y/prefer-tag-over-role | L | exact | parity |
| javascript:S7758 | eslint | unicorn/prefer-code-point | L | exact | parity |
| javascript:S7761 | eslint | unicorn/dom-node-dataset | L | exact | parity |
| javascript:S7764 | eslint | unicorn/prefer-global-this | L | exact | parity |
| javascript:S7765 | eslint | unicorn/prefer-includes | L | exact | parity |
| javascript:S7781 | eslint | unicorn/prefer-string-replace-all | L | exact | parity |
| javascript:S7784 | eslint | unicorn/prefer-structured-clone | L | exact | parity |
| javascript:S7785 | eslint | unicorn/prefer-top-level-await | L | exact | parity |
| javascript:S7787 | eslint | unicorn/require-module-specifiers | L | exact | parity |
| javascript:S930 | eslint | sonarjs/no-extra-arguments | L | exact | parity |
| python:S1066 | ruff | SIM102 | H, L | close | parity |
| python:S1135 | ruff | FIX002 | H, L | close | parity |
| python:S117 | ruff | N802 | H, L | close | parity |
| python:S117 | ruff | N806 | H, L | close | parity |
| python:S1172 | ruff | ARG001 | H, L | close | parity |
| python:S125 | ruff | ERA001 | H, L | close | parity |
| python:S1481 | ruff | F841 | H, L | close | parity |
| python:S1523 | ruff | S102 | H, L | exact | parity |
| python:S1523 | ruff | S307 | H, L | exact | parity |
| python:S2068 | ruff | S105 | H, L | close | parity |
| python:S2068 | ruff | S106 | H, L | close | parity |
| python:S2068 | ruff | S107 | H, L | close | parity |
| python:S2077 | ruff | S608 | H, L | close | parity |
| python:S2245 | ruff | S311 | H, L | close | parity |
| python:S3776 | ruff | C901 | H, L | close | parity |
| python:S3776 | ruff | PLR0912 | H, L | close | parity |
| python:S4721 | ruff | S602 | H, L | close | parity |
| python:S4721 | ruff | S605 | H, L | close | parity |
| python:S4790 | ruff | S324 | H, L | close | parity |
| python:S4830 | ruff | S501 | H, L | close | parity |
| python:S5042 | ruff | S202 | H, L | close | parity |
| python:S5247 | ruff | S701 | H, L | close | parity |
| python:S5443 | ruff | S108 | H, L | close | parity |
| python:S5754 | ruff | BLE001 | H, L | close | parity |
| python:S5754 | ruff | E722 | H, L | close | parity |
| python:S5806 | ruff | A001 | H, L | close | parity |
| python:S6903 | ruff | DTZ003 | H, L | close | parity |
| python:S7494 | ruff | C402 | H, L | close | parity |
| python:S7494 | ruff | C408 | H, L | close | parity |
| python:S7519 | ruff | C420 | H, L | exact | parity |
| python:S8495 | pyright | reportAssignmentType | L | close | parity |
| typescript:S3735 | eslint | sonarjs/void-use | L | exact | parity |
| typescript:S4325 | eslint | @typescript-eslint/no-unnecessary-type-assertion | L | exact | parity |
| typescript:S5332 | eslint | sonarjs/no-clear-text-protocols | L | exact | parity |
| typescript:S6845 | eslint | jsx-a11y/no-noninteractive-tabindex | L | exact | parity |

The native checks of the surface family run in scan, the staged scan and the agent hooks, without any external tool:

| Check | Sonar key | Answer |
| --- | --- | --- |
| surface.code-eval | javascript:S1523, python:S1523 | denies when the candidate introduces it |
| surface.hardcoded-credential | javascript:S2068, python:S2068, docker:S6472 | denies when the candidate introduces it |
| surface.file-length | none | denies when the candidate introduces it |
| surface.inline-script-size | none | denies when the candidate introduces it |
| surface.suppression | none | denies when the candidate introduces it |
| surface.analyzer-widening | none | denies an agent write that widens the scope |
| surface.html-prefer-tag | Web:S6819, javascript:S6819 | feedback in the hook, a failure in scan |
| surface.python-function-length | none | feedback in the hook, a failure in scan |
| docker.tag-and-digest | docker:S8431 | denies when the candidate introduces it |
| docker.final-user | docker:S6471 | feedback in the hook, a failure in scan |
| cfn.bucket-https-only | cloudformation:S6249 | feedback in the hook, a failure in scan |

Coverage on new code maps to golaw coverage --new-code, exact by model: Sonar's line model is ported, see below.

## The gaps

No local rule and no native check covers these keys, so a finding of theirs is left to the pipeline, with the fix that cleared it by hand:

- docker:S7020, a long line: split the RUN instruction.
- docker:S6470, a recursive COPY of the build context: copy the paths the image needs.
- docker:S6504, executables not owned by root: give root the ownership and the user only the run permission.
- docker:S6505, install scripts allowed: install with npm ci --ignore-scripts.
- docker:S2612, a world writable chmod: grant only the owner and the group.
- python:S1313, a literal IP address (ruff S104 covers only the 0.0.0.0 bind): read the address from configuration, or use a documentation range in tests.
- python:S5852, a regular expression open to backtracking: parse linearly.
- python:S1192, a duplicated literal: name it once.
- cloudformation:S6304 and cloudformation:S6302, IAM wildcards: name the actions and the resources; their checkov ids join the parity set once verified on fixtures.
- Web:S5254 and Web:S1077 are in the strict set through @html-eslint and claim no parity until a profile snapshot confirms them.
- javascript:S1523 cleared by writing code to a temporary file and requiring it is dynamic loading by another name, and no certain predicate recognizes it: export the code a test needs and call it.

The docker and cloudformation gaps with a crisp syntactic predicate, a COPY . . before the last USER or a chmod that grants others write, are candidates for native checks once the Sonar predicate is read.

## The toolchain

External linters are pinned by version and checksum and never built into the binary. golaw setup --surface downloads and verifies the tools the languages of the repository need, golaw setup --surface=all every tool; it is the only command that reaches the network for them, run by a person or by CI. A version enters seven days after its publication.

| Tool | Version | Distribution | Languages | Hook |
| ---- | ------- | ------------ | --------- | ---- |
| cfn-lint | 1.58.0 | pypi | cloudformation | no |
| checkov | 3.3.30 | pypi | dockerfile, cloudformation | no |
| eslint | 10.13.0 | npm | javascript, typescript, jsx, tsx, html | no |
| hadolint | 2.15.1 | binary | dockerfile | yes |
| pyright | 1.1.414 | npm | python | no |
| ruff | 0.17.1 | binary | python | yes |
| shellcheck | 0.11.0 | binary | shell | yes |
| shfmt | 3.14.1 | binary | shell | no |
| stylelint | 17.16.0 | npm | css, html | no |
| yamllint | 1.38.0 | pypi | yaml | no |

The tools live outside the checkout, in $XDG_DATA_HOME/golaw/tools when it is set, %LOCALAPPDATA%\golaw\tools on Windows and ~/.local/share/golaw/tools elsewhere, macOS included, each with a receipt of its path, version, sha256, size and modification time. Node 22.13 or newer and Python 3.10 to 3.14 are prerequisites golaw never installs. A tool on PATH whose version equals the pin is used too, reported as checksum not verified. A missing tool is a doctor line and a notice, never a denial.

## The hook

The pre event of a write runs the native checks alone, in process. The post event runs at most one fast tool on the file just written, with its bytes on stdin: ruff on Python, hadolint on a Dockerfile, shellcheck on a shell script; ESLint, stylelint, pyright, cfn-lint and checkov never run in a hook. Every tool is killed at 1.5 s, and the external work of one event stops at 3 s, inside the hook's own budget. A killed tool yields the notice not measured, the pipeline judges it. The acceptance budget is a post event on one file of 300 lines under 1 s at the 95th percentile of 20 warm runs with its tool installed, and a pre event under 300 ms.

The certain checks deny only a violation the candidate introduces, and their reason states the fix; every tool finding and every check that is not certain goes back to the agent as feedback, at most twenty per file with the rest counted, as path:line rule (Sonar key) message, and never blocks.

## The lint and CI targets

golaw lint --surface runs every pinned tool for the languages the snapshot holds over the files of the analyzer scope, test files under the main-code rules when the scope names no tests, and reports in the text and JSON shapes of golaw scan with the Sonar key of each mapped finding. It exits 0 clean, 1 with findings, and 2 when a required tool is missing or a file is left unmeasured, because the lint target and CI never answer clean without measuring. The golaw.mk of an adopted repository gains lint-surface and coverage-new, and its CI template a surface job that runs golaw setup --surface with the tool directory cached by the kit version, which pins one toolchain.

## Coverage on new code

golaw coverage --new-code --input profile.cov reproduces the number the pipeline's gate reads: the report written as the pipeline writes it (go test -coverprofile profile.cov ./..., no -coverpkg, mode set), Sonar's Go line model (block edges trimmed of braces, blank lines ignored, hits merged per line), the Go files outside _test.go minus the exclusions and coverage exclusions of the analyzer scope and the generated files, and the lines git diff --unified=0 names against --base <ref> or, with --since-tag, the last tag. The floor is checks.coverage.new_code, 80 by default. No new line to cover is not applicable, never 100; a missing, empty, unparseable or stale report is unmeasured and exits 2.

## The kit's own tree

The kit's own make ci runs lint-surface and coverage-new over its own tree, as policy.ci-target requires of every repository. Its non-Go files are fixtures under testdata, its own configuration tree under internal/lint/config, which the kit's golaw.yml leaves outside its analyzer scope, and files of tools with no parity rule, so lint-surface needs no tool there; the native checks of the self scan judge all of them.
