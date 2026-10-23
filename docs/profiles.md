# Profiles

A profile is data, never code. Core only is the default; a profile is selected explicitly and recorded in `golaw.yml`. The remote, module path and CODEOWNERS never select one implicitly. M3 validates the identity stamp and reference; payload loading and profile install belong to M5. M5 builds them: a person installs a bundle from its git source at a tag, the kit verifies it before it writes, every governed operation loads the pinned data from the installed copy, and the pin of a repository moves only through a reviewed, committed init.

A profile adds checks or tightens floors; it never relaxes a core rule. This keeps the core a dependable minimum across repositories. Template subtraction happens only during init. The profile contract permits omitting template files another team owns, such as files under `.github/`, without removing checks or changing policy during a hook. Confining it to init keeps the omission a reviewed diff a person applies, so no hook changes which files a repository holds. Under a profile whose `templates.exclude` names a destination, init writes nothing there and names each one after the line `Deixados para a plataforma:`.

## The bundle

A bundle is a directory of a git repository, one tag per version. `profile.yml` is its identity and `rules.yml` its data; every other regular file, such as the onboarding page, a README or fixtures, is data the kit never reads. The digest is `sha256:` and the lowercase hex of the SHA-256 over `<slash path>\n<sha256 hex of the bytes>\n` for every regular file of the bundle except `profile.yml`, sorted by path, with a `.git` directory left out. The onboarding page is covered too, so the page a developer reads cannot change under a pin without a trace. `golaw profile digest <dir>` validates `rules.yml` of a directory and prints that digest, the value its author writes into the stamp before tagging. Run it over a clean clone of the commit to tag, because every regular file counts, ignored ones included: build output, a `.DS_Store` or a coverage profile left in a working checkout, or the `.git` file at the root of a linked worktree, gives a digest no clone of the tag reproduces, and every install of that tag then refuses it as a digest mismatch.

## Identity fields and validation

The bundle's `profile.yml` is `policy.Stamp`: exactly five fields. The `profile` mapping in `golaw.yml` is `policy.ProfileRef`: exactly four fields, with no `date` key. All fields of a present stamp or reference are required. Omitting the reference selects core only for a repository that has not enrolled a profile; it is not a way to remove an enrolled policy.

| Field | Bundle identity | Repository reference | Current validation |
| --- | --- | --- | --- |
| `name` | Required | Required | Lowercase ASCII name matching `^[a-z][a-z0-9-]*$`: a letter first, then lowercase letters, digits or hyphens. |
| `version` | Required | Required | Exact `X.Y.Z` numeric pin, with no leading zeroes except a component equal to zero. No `v` prefix, range, prerelease suffix or `latest`. |
| `date` | Required | Absent | Valid calendar date in `YYYY-MM-DD` form, such as `2026-10-20`. |
| `source` | Required | Required | Nonempty string, the canonical git URL of the bundle. Install records it from the stamp, never from the URL a person typed. |
| `digest` | Required | Required | `sha256:` followed by exactly 64 lowercase hexadecimal characters, recomputed over the files at install and at every load. |

Unknown and duplicate keys are invalid in both schemas. Invalid identity fields are policy errors, distinct from an evaluator's unsupported input. The date belongs only to the stamp: doctor reports the days since it, and no age refuses a bundle.

Both examples below are the fixture bundle of the kit, `internal/profile/testdata/fixture`: its stamp carries the digest of its own files, and its source is a synthetic address no command fetches. The repository example's `kit.version` is a separate kit pin, not a profile version.

## Bundle identity

```yaml
name: fixture
version: 1.2.3
date: 2026-10-20
source: https://example.test/profiles/fixture
digest: sha256:e089c40003920fad658fdaa375c34e9229e987fd6af163511d2d3df8d9690c96
```

## Repository reference

```yaml
golaw: 1
kit: { version: 0.5.0 }
capabilities: [go, image]
profile:
  name: fixture
  source: https://example.test/profiles/fixture
  version: 1.2.3
  digest: sha256:e089c40003920fad658fdaa375c34e9229e987fd6af163511d2d3df8d9690c96
```

## The rules schema

`rules.yml` is one YAML document whose first key is `golaw_profile: 1`, decoded strictly: an unknown or duplicate key is a schema error. Every section is optional and additive, and the schema has no key that names a core floor, so a profile cannot relax one by construction. Validation refuses an empty section, a glob or pattern that does not compile, a stage named twice, an assertion without a kind, and a target or term that is not lowercase kebab case.

| Section | Holds | Read by |
| --- | --- | --- |
| `terms` | Extra forbidden terms, merged into the tool terms of the policy. | text.tool-terms |
| `paths` | Forbidden path globs over the whole path from the root; a trailing slash covers a tree. | paths.profile-forbidden |
| `layout` | The `main` and `decisions` defaults init renders when the repository names no layout and holds no decisions directory. | init |
| `templates.exclude` | Destination prefixes init leaves out. | init |
| `targets` | Make targets the ci target must carry beyond the core table of rules.md. | policy.profile-targets |
| `dockerfile` | The ordered `stages`, the `final_image`, the `installs` allowlist of each stage, `pinned_installs` and the forbidden `ignore_comments` prefixes. | policy.image-stages, policy.image-installs |
| `registry` | The `pattern` every image of a FROM line must match. | policy.image-registry |
| `assertions` | Typed YAML assertions: the `path`, the `key` path, the `kind` (`bool`, `string` or `int`), the optional `equals`, what a `missing` key is, and whether the file is `required` or `optional`. | policy.values-assertions |

### Example: the fixture rules.yml

```yaml
golaw_profile: 1
terms: [legacyci]
paths: ["AGENT-NOTES.md", ".agentconfig/"]
layout: { main: root, decisions: docs/adr }
templates: { exclude: [".github/"] }
targets: [hooks-test, image-lint]
dockerfile:
  stages: [certs, base, ci, builder]
  final_image: scratch
  installs:
    - { stage: ci, packages: [ca-certificates, git, tzdata] }
  pinned_installs: true
  ignore_comments: ["# lint ignore"]
registry: { pattern: "^registry\\.example\\.test/" }
assertions:
  - {
      path: deploy/values.yaml,
      key: [securityContext, runAsNonRoot],
      kind: bool,
      equals: "true",
      missing: finding,
      files: required,
    }
  - {
      path: deploy/values.yaml,
      key: [podAnnotations, example.test/do-not-disrupt],
      kind: string,
      equals: "true",
      missing: finding,
      files: required,
    }
  - {
      path: "deploy/jobs/*.yaml",
      key: [spec, activeDeadlineSeconds],
      kind: int,
      missing: finding,
      files: optional,
    }
```

## Where bundles live

`<config>/profiles/<name>/` is the current bundle of a name, under the per-OS configuration directory. `<config>/profiles/<name>@<version>/` is the store, one immutable directory per installed version. `<state>/profiles/<name>.json` is the install record, under the per-OS state directory: every installed version with its digest, the URL and tag it came from and when, and which one is current. The loader reads the current stamp first and, when its version or digest differs from the pin, the stored copy at the pinned version, so two repositories pinned at two versions of one profile both keep working on one machine.

## Install and lifecycle

Every lifecycle command is run by a person, prints pt-BR lines and exits 0 when the step is done or was already done, 2 otherwise with one `golaw: profile <sub>: <cause>` line. None of them reads or writes a `golaw.yml`.

```sh
golaw profile install <git url>@<tag>
golaw profile refresh <name>
golaw profile rollback <name>
golaw profile remove <name>
```

`install` clones the tag with git and the person's own git configuration, `vX.Y.Z` or `X.Y.Z` naming the same version. It verifies before it writes: the stamp, the digest recomputed over the files, `rules.yml` decoded and validated, and the stamp's version equal to the tag's. It then copies the bundle into the store, makes it current with one rename and records it, printing `Perfil <name> <version> instalado de <url>@<tag>, digest <digest>`. A version already stored with the same digest copies nothing and prints `Perfil <name> <version> já estava instalado`. The same version with another digest is refused, because a published version never changes. When the URL given is a mirror, the record keeps it while the stamp's source stays the canonical one the pin names.

`refresh` installs the current version again from the URL and tag its record names; it is the way out of a current bundle whose files changed. `rollback` makes the previously current stored version current again and prints `Perfil <name> voltou para <version>`. `remove` deletes the current bundle, every stored version and the record, and prints `Perfil <name> removido`.

## Adopt a profile and move the pin

`golaw init --profile <name>` reads the stamp of the installed current bundle, never the network, and previews the `profile:` line of `golaw.yml`; `--apply` writes it, and the person commits it. Without a bundle installed for that name, init refuses and names the install command, because the kit cannot know the source. To move a pin, install the other version (or roll back to it), run `golaw init --profile <name> --apply`, commit, then run `golaw init --apply` once more to enroll the committed pin: the diff changes the version and the digest of the pin and nothing else, and until the second init every governed operation refuses the committed pin as an enrollment mismatch. A teammate who pulls a moved pin installs that version with the command the pin names and runs `golaw init --apply` to enroll it. Refresh, rollback and install alone never change a pin, and installing another version cannot make a mismatched pin valid.

## A missing or different bundle

An absent or mismatched profile never falls back to core. Doing so would silently replace the enrolled policy with weaker rules. Every governed operation refuses with one line naming the exact command: scan, scan --staged and scan --enrolled exit 2 with one `golaw: ` line, an armed rung refuses with it, init --apply, agent install, setup, coverage and config explain refuse with it, and an installed agent hook on an enrolled repository denies with the English line as the reason and `Perfil <name> ausente ou divergente: execute golaw profile install <source>@<version>` for the person. The lines are:

- `profile <name> is not installed: run golaw profile install <source>@<version>`
- `profile <name>: <field> is <pinned> in golaw.yml and <installed> in the installed bundle: run golaw profile install <source>@<version>`
- `profile <name>: digest <computed> of the installed bundle differs from its stamp <stamped>: run golaw profile install <source>@<version>`
- `profile <name>: rules.yml: <cause>`, which names no command because installing the same bundle again cannot cure it.

A location that is positively unenrolled gets the valid allow of the hook whatever its `golaw.yml` names, so a fresh clone never bricks. Hooks never fetch a profile.

doctor reports the pinned profile on one line: `profile-none` for core only, `profile-installed` with `version <v>, digest <d>, <n> days since <date>`, and as red `profile-not-installed`, `profile-version-mismatch`, `profile-digest-mismatch` and `profile-stale`, each naming the install command, and `profile-data-invalid` naming the schema cause.

## Evaluators

Seven generic checks of the core read the data of a profile. A profile needing new logic requires a generic core evaluator; it cannot execute bundle code. Each check answers NotApplicable without a profile and when the profile carries no section for it, and applies only where its capability holds; a missing capability makes it inapplicable, rather than a finding. The synthetic fixture bundle and the fixtures of internal/proof exercise every evaluator with a red case and a near miss, and no platform value.

| Check | Section | Needs | Finding codes | Stated limits |
| --- | --- | --- | --- | --- |
| `policy.image-stages` | `dockerfile.stages`, `dockerfile.final_image` | image | `image-stage-missing`, `image-stage-order`, `image-final-image` | Only the root `Dockerfile` is judged, through the Dockerfile reader of rules.md. A last stage that starts from an earlier stage is judged by the image that stage resolves to. |
| `policy.image-installs` | `dockerfile.installs`, `dockerfile.pinned_installs`, `dockerfile.ignore_comments` | image | `image-install-not-allowed`, `image-install-unpinned`, `image-inline-ignore` | Reads `apk add`, `apt-get install` and `apt install` with plain package words, joined continuations included; a variable, a substitution, an option with an argument, an exec form install, a heredoc install, and an install behind an assignment, a shell keyword, a subshell, sudo, a path or an option that takes an argument before its verb are stated limits. Each clears by the rewrite the reader table of rules.md names: write each install as its own plain `apk add`, `apt-get install` or `apt install` command, move an assignment to `ENV` or `ARG`, drop sudo, the path, the subshell and the `if`, put an option that table does not list before the verb, drop an option that takes an argument, write each package as `name` or `name=version` with no pipe, redirection or OR list after it, and write the install in the shell form outside a heredoc. A comment prefix compares without the mark, case and extra spaces, and matches only where a word ends, so prose whose word merely starts with the prefix is not a match. |
| `policy.image-registry` | `registry.pattern` | image | `image-registry` | Only the root `Dockerfile` is judged. |
| `policy.profile-targets` | `targets` | go | `profile-target-missing`, naming `add <target> to the prerequisites of ci` | The ci closure and the scheduled or manual job allowance of policy.ci-target, with the Makefile reader's limits. |
| `policy.values-assertions` | `assertions` | none | `values-assertion`, `values-key-missing`, `values-file-missing` | A parent that is not a mapping is a missing key; the YAML reader of rules.md reads the file. A plain YAML 1.1 boolean (`yes`, `no`, `on`, `off`, `y`, `n`, in any case) where a bool is asserted, which Helm reads as a boolean, and a mapping on the key path whose key is not a scalar, such as a template expression, are stated limits. Each clears by a rewrite: write `true` or `false` for a bool, and write every key of the mappings on the key path literally instead of a template expression. |
| `paths.profile-forbidden` | `paths` | none | `profile-forbidden-path` | A glob matches the whole slash path from the root, a trailing slash covers a tree, names compare without case, and a file under testdata is never judged. Without case a name a case-insensitive filesystem aliases is the forbidden one (A14), and a case-sensitive filesystem is judged the same: a name that differs from a forbidden one in case alone is refused there too. An agent write is denied before the file exists. |
| `text.tool-terms` | `terms` | none | the codes of the check | A policy whose own tool list holds only the profile's terms keeps the kit's default terms. |

Unsupported syntax or unavailable external evidence is Unmeasured, never a text match. A text match over a construct the reader does not understand would give false findings and false clean results alike, so the check states its limit instead. A verdict that depends on a construct outside the declared subset names the file, line and construct as a stated limit. Unrelated unsupported syntax does not broaden the reader. An armed rung names the limit and leaves it to the pipeline; a required unmeasured result cannot establish a clean remote result and is not itself a finding.

## Remote gate and local choice

A profile's remote gate is whatever the adopting platform runs. Without a remote stage, automatic kit checks run only on machines whose developers armed them. Optional manual scans such as `golaw scan .` remain available without arming git hooks; their results describe that scan, not remote enforcement. Agent guards are separately opted into by installation.

Every local check is optional. Only the developer arms git rungs with `golaw setup`; `init` alone never arms one. Each rung switches off in `golaw.yml` without a finding. Armed rungs finish in seconds, pre-push runs govulncheck only, and the full `make ci` belongs in remote CI or the platform pipeline, the only gate. A profile cannot make those local aids mandatory.

A remote hard gate requires a demonstrated remote rejection. A local clean scan, an installed profile or a checked-in CI file does not demonstrate that the adopting platform rejects a violating change. Record the remote revision, exercised violation and rejection before claiming enforcement; an absent remote stage remains a stated limitation.

Platform-specific onboarding, evidence requirements and values stay outside the public kit. Public examples and evaluator fixtures remain synthetic; private profile fixtures run in their owning repository against the released public binary. See the [checklists](checklists.md) for evidence records and [security guide](security.md) for assurance limits.
