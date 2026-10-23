# Codex live record on macOS

Offline acceptance proves the Codex adapter and its installed command. Only a live session proves that Codex runs the hook, honors its trust record and stops an `apply_patch` write before the disk. This page is the procedure of that session. Its record is `docs/live/codex-darwin.json`, which `go run ./tools/acceptance -m4` and `go run ./tools/acceptance -release` read: the rows `live codex-denied-write` and `live codex-allowed-control` pass only when the record names the darwin/arm64 candidate of the commit under test.

## What the lane does

`go run ./tools/acceptance -live-codex facts.json`, run on a macOS arm64 machine:

1. Builds the candidate exactly as the release gate rebuilds it (`CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -buildvcs=false ./cmd/golaw`) and records its SHA-256.
2. Prepares a temporary home with its own `CODEX_HOME` and a temporary git repository enrolled with `golaw init --apply`, so no file of your own Codex configuration is read or written.
3. Installs the hook with `golaw agent install --agent codex` in that home, writes the `hooks.state` trust record of each installed handler into that home's `config.toml`, the record the `/hooks` review writes, and stops unless `golaw doctor` reports the hook trusted.
4. Runs `codex exec` twice in the repository with a prompt pinned to the `apply_patch` tool: first adding `notes/denied.md` with a line that holds an em dash (U+2014), then adding `notes/allowed.md` with the line `one-two`.
5. Records `denied-write` as blocked with the target absent only when `notes/denied.md` does not exist afterwards, the session's own output shows Codex blocking the patch of that file (`blocked by PreToolUse hook` beside `notes/denied.md`) and `golaw doctor` shows the hook dispatched for the repository, and `allowed-control` as allowed with the target present only when `notes/allowed.md` holds `one-two`.
6. Writes `docs/live/codex-darwin.json` when both cases hold, and nothing otherwise.

## What you provide

- The Codex CLI at the validated version on PATH: `codex --version` prints `codex-cli 0.163.0`.
- A credential in the `CODEX_API_KEY` variable of the shell that runs the lane, an API key with a spending limit. The lane hands it to `codex exec` through the environment only; it never prints it or writes it, and the temporary home is deleted when the lane ends.
- A facts file, a JSON object with the keys `date` (the day of the session, as 2026-10-21) and `owner`, `environment`, `credential_boundary`, `spending_limit`, `schedule` and `retention`, each a sentence: the facts every live record keeps. The date enters through the file because the harness reads no clock.

## When the lane cannot run unattended

No acceptance run holds a credential: the isolated home of every run is empty. Until a person runs this procedure, `-m4` reports `live codex-denied-write` and `live codex-allowed-control` as SKIP with that reason, and `-release` refuses the SKIP. Without `CODEX_API_KEY`, or off darwin/arm64, the lane itself answers SKIP with its reason and writes nothing.

## After the session

Review `docs/live/codex-darwin.json` and commit it alone before the tag: a commit that only adds the record keeps the candidate digest. Record a new session whenever the toolchain, the candidate or the validated Codex version changes. Cursor has no lane yet: it stays a compatibility preview until an `agent -p` lane records its hooks firing.

## The dispatched job

`.github/workflows/live-codex.yml` runs this procedure on a hosted macOS arm64 runner when a person dispatches it (`workflow_dispatch`), never on a push or a pull request. The seven facts are its required inputs. It needs the repository secret `CODEX_API_KEY`, an API key with a spending limit that the owner provisions before the first run; without it the lane answers SKIP and the job fails, because no record exists to keep. The job prints the record in its log and its summary; review it and commit it alone, as above.
