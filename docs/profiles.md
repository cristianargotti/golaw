# Profiles

A profile is data, never code. Core only is the default; a profile is selected explicitly and recorded in `golaw.yml`. The remote, module path and CODEOWNERS never select one implicitly. M3 validates the identity stamp and reference; payload loading and profile install belong to M5.

A profile adds checks or tightens floors; it never relaxes a core rule. This keeps the core a dependable minimum across repositories. Template subtraction happens only during init. The profile contract permits omitting template files another team owns, such as files under `.github/`, without removing checks or changing policy during a hook. Confining it to init keeps the omission a reviewed diff a person applies, so no hook changes which files a repository holds. Loading that profile data is part of M5, not an M3 template-subtraction API.

## Identity fields and validation

The bundle's `profile.yml` is `policy.Stamp`: exactly five fields. The `profile` mapping in `golaw.yml` is `policy.ProfileRef`: exactly four fields, with no `date` key. All fields of a present stamp or reference are required. Omitting the reference selects core only for a repository that has not enrolled a profile; it is not a way to remove an enrolled policy.

| Field | Bundle identity | Repository reference | Current validation |
| --- | --- | --- | --- |
| `name` | Required | Required | Lowercase ASCII name matching `^[a-z][a-z0-9-]*$`: a letter first, then lowercase letters, digits or hyphens. |
| `version` | Required | Required | Exact `X.Y.Z` numeric pin, with no leading zeroes except a component equal to zero. No `v` prefix, range, prerelease suffix or `latest`. |
| `date` | Required | Absent | Valid calendar date in `YYYY-MM-DD` form, such as `2026-10-20`. |
| `source` | Required | Required | Nonempty string. The current validator does not validate a URL, transport or whether the source can be reached. |
| `digest` | Required | Required | `sha256:` followed by exactly 64 lowercase hexadecimal characters. |

Unknown and duplicate keys are invalid in both schemas. Invalid identity fields are policy errors, distinct from an evaluator's unsupported input. The date belongs only to the stamp: M3 validates the calendar date but does not impose an age or staleness threshold.

Both examples below are synthetic schema examples. The zero digest demonstrates syntax and is not the verified digest of an installable bundle. The source is illustrative, not a bundle to fetch. The repository example's `kit.version` is a separate kit pin, not a profile version or an upgrade recommendation.

## Bundle identity

```yaml
name: fixture
version: 1.2.3
date: 2026-10-20
source: https://example.test/policies
digest: sha256:0000000000000000000000000000000000000000000000000000000000000000
```

## Repository reference

```yaml
golaw: 1
kit: { version: 0.2.0 }
capabilities: [go]
profile:
  name: fixture
  source: https://example.test/policies
  version: 1.2.3
  digest: sha256:0000000000000000000000000000000000000000000000000000000000000000
```

## What M3 verifies

`policy.Decode` and `policy.File.Validate` validate the repository configuration and its reference. When policy loading encounters a reference, `policy.VerifyProfile` reads `profiles/<name>/profile.yml` under the per-OS configuration directory through `policy.LoadStamp`. It validates the stamp and compares `name`, `source`, `version` and `digest` exactly against the reference. The date is validated separately because the reference has none.

This verifies identity agreement, not payload integrity: M3 does not hash evaluator payload files or load a verified cached bundle. A syntactically valid digest that matches the stamp is not evidence that any bundle content was verified.

An absent or mismatched profile never falls back to core. Doing so would silently replace the enrolled policy with weaker rules. A missing stamp yields a policy error naming the intended install command; a mismatch names the field. The installer named by that diagnostic is future M5 behavior, not an available M3 command. Hooks never fetch a profile.

## Evaluator semantics

The profile design supplies data to generic evaluators in the core. These categories describe semantics, not serialized payload keys or an executable schema that M3 defines:

| Category | Meaning |
| --- | --- |
| Image stages and installation | Required ordering of image stages and allowlists for what tools or packages a stage may install. |
| Targets and hook commands | Required make targets and permitted hook command structure, subject to the optional local-rung contract. |
| Typed YAML assertions | Assertions on typed values with an explicit outcome for a missing key, not a match against raw text. |
| Properties precedence | Properties interpreted with a declared precedence when more than one source can supply a value. |
| Protected-base ownership | Ownership decisions taken from the protected base, including the catch-all owner, so a candidate edit cannot authorize itself. |
| Forbidden terms | Additional terms the profile forbids under the core's matching and scope rules. |
| Registry patterns | Allowed image registry patterns supplied as profile data. |
| Branch patterns | The adopting repository's branch-name pattern. |
| Layout defaults | Default locations for the main package and decision records. |

A profile needing new logic requires a generic core evaluator; it cannot execute bundle code. Checks apply only where their capability holds. A missing capability makes a check inapplicable, rather than a finding.

Unsupported syntax or unavailable external evidence is Unmeasured, never a text match. A text match over a construct the reader does not understand would give false findings and false clean results alike, so the check states its limit instead. A verdict that depends on a construct outside the declared subset names the file, line and construct as a stated limit. Unrelated unsupported syntax does not broaden the reader. An armed rung names the limit and leaves it to the pipeline; a required unmeasured result cannot establish a clean remote result and is not itself a finding.

## Future M5 installation lifecycle

M5 will add payload loading, verified cached bundles and the human-run installation lifecycle. The following steps describe that future behavior, not commands available in M3:

1. A person explicitly runs `golaw profile install <git url>@<tag>` to install the selected bundle in the per-OS configuration directory and record its exact version and digest. No repository identity triggers discovery or installation.
2. Explicit adoption through `golaw init --profile ... --apply` records the source, exact version and digest in a reviewed policy change that the person commits. Installation alone does not silently select a repository's policy.
3. Hooks use only digest-verified cached bundles and never fetch during a hook. An unavailable bundle prevents the governed operation with a reason naming the exact installation command; it cannot silently remove the profile's checks.
4. Refresh and rollback never change an enrolled pin implicitly. A pin change requires the explicit reviewed adoption step and commit, with the matching bundle available and the core floors preserved. Installing an older or newer bundle alone cannot make a mismatched reference valid.
5. Doctor reports the profile's version, digest and age. The adopting platform supplies its own CI provisioning and bundle-verification evidence; the public kit does not prescribe private mounts, credentials or platform values.

## Remote gate and local choice

A profile's remote gate is whatever the adopting platform runs. Without a remote stage, automatic kit checks run only on machines whose developers armed them. Optional manual scans such as `golaw scan .` remain available without arming git hooks; their results describe that scan, not remote enforcement. Agent guards are separately opted into by installation.

Every local check is optional. Only the developer arms git rungs with `golaw setup`; `init` alone never arms one. Each rung switches off in `golaw.yml` without a finding. Armed rungs finish in seconds, pre-push runs govulncheck only, and the full `make ci` belongs in remote CI or the platform pipeline, the only gate. A profile cannot make those local aids mandatory.

A remote hard gate requires a demonstrated remote rejection. A local clean scan, an installed profile or a checked-in CI file does not demonstrate that the adopting platform rejects a violating change. Record the remote revision, exercised violation and rejection before claiming enforcement; an absent remote stage remains a stated limitation.

Platform-specific onboarding, evidence requirements and values stay outside the public kit. Public examples and evaluator fixtures remain synthetic; private profile fixtures run in their owning repository against the released public binary. See the [checklists](checklists.md) for evidence records and [security guide](security.md) for assurance limits.
