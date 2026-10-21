# Contributing

The repository has a small set of laws. `make ci` checks every one a machine can check, and remote CI runs the same target on macOS, Linux and Windows; it is the only gate. Nothing on a developer's machine is mandatory: the rungs in `lefthook.yml` run only after `golaw setup` arms them, which nobody runs for you, and `golaw setup --uninstall` removes them, as [docs/setup.md](docs/setup.md) describes.

## Laws

- Go files have at most 300 lines and functions at most 40 lines, counted the way funlen counts with comments ignored. Test files keep the file limit and lose the function limit.
- No em dash and no en dash anywhere, source and documentation included. Use a comma, a colon or a hyphen.
- Comments are in English and say why, not what. Strings of the human channel (the messages a hook shows to the person) are in the configured language, pt-BR by default; everything else is English.
- Coverage is 80 percent in total, per package and per file, with the 20 statement minimum and no exemption list.
- Every check ships with a red fixture and a near-miss fixture bound through `internal/proof/manifest.yml`; the proof test fails when a registered check has no entry or an entry has no registered check.
- Every direct dependency has a decision record under `docs/adr/` naming its module path; a version enters seven days after publication. The standard library comes first.
- Commits are conventional and in English: `feat(scope): what changed`, `fix`, `docs`, `test`, `build`, `ci`, `chore`, `refactor`, `perf`, with `!` for a breaking change. No trailer of any kind: no sign-off, no co-author line, no generated-with line.
- The public tree names no company, carries no session traces, no absolute home paths and no trailers. Agent and vendor names appear only under `agents/`, `.github/`, `docs/`, the README, this file, the marketplace pointer under `.claude-plugin/`, `golaw.yml`, whose term list names them, and the narrow source scopes the self scan allows; an allowed path that ends in a slash is a directory, any other is one file.
- Hooks invoke the binary, never a shell script.
- A reachable vulnerability found by the scheduled lane (`.github/workflows/vuln.yml`) is fixed by a toolchain or dependency bump. When no fix exists yet, a dated waiver in `vulnerability_waivers` of `golaw.yml` with its decision record keeps the lane green until the day it names, never longer; the pull request gate never depends on the live database.

## Proposing a change

1. Open an issue that states the problem and the command or rule it touches.
2. Branch from `main`, keep the change small, write the failing test first, then the code, then run `make ci`.
3. Open a pull request with the conventional title; CI must be green on the three operating systems.
