# golaw check

`golaw check [path] [--base <ref>] [--json] [--verbose] [--timeout <duration>]` runs, in one command, the gates `make ci` fails on, restricted to what the worktree changed, and prints only what is red. An agent runs it once after it writes, instead of running `go test`, `go vet`, `golangci-lint run`, `golaw scan` and the coverage commands one turn at a time: a benchmark of agents doing real tasks on an enrolled service counted about 21 verification turns in about 90 per task, each one rereading the whole context.

It is a local aid like every other: nothing arms it, nothing requires it, and the pipeline stays the only gate.

## The change it judges

The change is everything the worktree holds since its fork from the base: the commits since the merge base, the staged and unstaged edits and the untracked files git does not ignore. The base is the one `golaw coverage --new-code` resolves, origin/HEAD, then origin/main, then origin/master, and `--base <ref>` names another; a named base git cannot resolve exits 2. When no base resolves, or the default one shares no history with HEAD as in a shallow clone, the whole tree is judged and the report says so in its second line.

The affected packages are the packages that hold a changed file and every package of the module that imports one of them, tests included; a change of go.mod or go.sum affects the whole module.

## The gates

The gates run at once as separate processes, under one budget, 10 minutes unless `--timeout` names another. None stops the others. Each gate that does not apply is skipped with a one-line reason: the capability is off, nothing it reads changed, or its tool is not on PATH.

| Gate       | What runs                                                                                                                                                 | Like                |
| ---------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------- |
| format     | `golangci-lint fmt --diff` over the changed Go files                                                                                                      | make fmt            |
| vet        | `go vet` over the affected packages                                                                                                                       | make vet            |
| lint       | `golangci-lint run --new-from-merge-base=<base>` over the affected packages with the repository's configuration; a changed configuration lints the module | make lint           |
| test       | `go test -json -coverprofile` over the affected packages                                                                                                  | make test           |
| coverage   | coverage on new code over that profile through the code of `golaw coverage --new-code`, counting the uncommitted lines too; skipped while a test is red   | make coverage-new   |
| scan       | the golaw rules over the worktree, as `golaw scan .`, keeping the findings of the changed files and those of no file                                      | make scan           |
| generate   | `golaw gen --check` when golaw.yml holds a generate block or `.golaw/gen.yml` exists                                                                      | make generate-check |
| migrations | `squawk` over the changed files of db/migrations under the db capability                                                                                  | make migrate-lint   |

The test gate runs without `-race` and `-shuffle`, so the go command can reuse its cache between runs; the pipeline still runs both. The total, package and file floors of `golaw coverage`, which read a profile of the whole module, stay in the pipeline.

## The report

The first line counts the gates. Each red gate follows with at most 12 lines of its failures and a count of the rest: for tests the failing test with its file, line and message, for lint, vet and scan the file, line, rule and message. Green gates print nothing unless `--verbose`; `--json` prints the same result as one document.

```text
golaw check: 8 gates, 3 green, 2 red, 3 skipped (base base)
red lint:
  fx.go:7:11: Error return value of `os.Remove` is not checked (errcheck)
red test:
  TestAdd: fx_test.go:7: Add = -1, want 3
skipped coverage: the tests are red
skipped generate: the repository declares no generation
skipped migrations: the db capability is off
```

It exits 0 when every gate is green or skipped, 1 when one is red, and 2 for a usage error or a run it could not start.
