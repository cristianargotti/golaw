---
record: 6
title: the pins of sqlc, oapi-codegen and goverter in the tools module
date: 2026-10-29
status: accepted
---

# 6. the pins of sqlc, oapi-codegen and goverter in the tools module

## Context

The generators of M7 write the shape of an enrolled repository from its contracts, and three of the contracts are read by pinned third party generators: sqlc reads the database schema and queries, oapi-codegen reads the OpenAPI document, and goverter writes the mappings between domain and adapter structs (M7.11.1). A version that drifts between two machines writes two different trees, so each generator needs one exact pin per kit version, chosen under the seven day dependency law on 2026-10-29.

The kit already pins lefthook, golangci-lint, gitleaks and govulncheck in the enrolled repository's `tools/go.mod` through the `tool` directive, with the hashes in its `go.sum`. Spec A14 accepts that one tools module beside the application module; it holds only if every pinned tool resolves and builds together in it.

## Decision

Pin the three generators in the enrolled repository's tools module, each guarded by the capability that needs it, so a repository without that capability never downloads the generator's module tree:

| Tool         | Tool line                                                  | Pin     | Capability  | Probe                   |
| ------------ | ---------------------------------------------------------- | ------- | ----------- | ----------------------- |
| goverter     | `github.com/jmattheis/goverter/cmd/goverter`               | v1.11.1 | `hexagonal` | `goverter version`      |
| oapi-codegen | `github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen` | v2.8.0  | `api`       | `oapi-codegen -version` |
| sqlc         | `github.com/sqlc-dev/sqlc/cmd/sqlc`                        | v1.31.1 | `db`        | `sqlc version`          |

The oapi-codegen runtime v1.7.0 is a require of the service module that the generated server imports, never of the tools module. `runner.Generators` lists the three rows apart from `runner.Toolchain`, because `golaw setup` verifies the rung toolchain on every machine and a repository without a database must never need sqlc. Each probe prints `v` followed by the pin, which the token match of the runner accepts.

The joint resolution was run on 2026-10-29 with go1.27.0 on darwin/arm64 and the default proxy, over one module holding the seven `tool` lines with every capability on (lefthook v2.1.12, golangci-lint v2.14.0, goverter v1.11.1, oapi-codegen v2.8.0, sqlc v1.31.1, gitleaks v8.30.1, govulncheck v1.7.0). `go mod tidy` settled `golang.org/x/tools` at v0.51.0, `golang.org/x/mod` at v0.41.0 and `github.com/getkin/kin-openapi` at v0.142.0, and golangci-lint, goverter, oapi-codegen and sqlc each built from that one module and printed its pin through its probe. A14 stays as it is.

## Alternatives

- Importing the generators into the kit: rejected by A55. The kit would carry their module trees into its own binary and its own dependency records, and a generator bump would become a kit release instead of a repository diff.
- One tools module per generator, `tools/<name>/go.mod`: not needed, because the joint resolution builds every tool from one module. It stays the fallback if a later pin bump breaks the joint resolution, and taking it amends A14, which is the owner's decision.

## Consequences

Dependabot moves the generator pins in the enrolled repository like any other tool. A pin bump in the kit reaches a repository as an `init --update` diff the person reviews (A10). oapi-codegen v2.8.1 enters under the seven day law after 2026-10-31, and the runtime v1.8.0 after 2026-10-30. The kit's own `go.mod` gains no module.
