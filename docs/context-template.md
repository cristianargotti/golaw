# Hexagonal Go context guide

Use this guide when the repository has the `hexagonal` capability. Add API, database, job and authentication shapes only when those concerns exist. The names describe a small context, not a framework or a new dependency requirement. Reuse the destination's established types and conventions and the [engineering checklists](checklists.md).

This guide does not cause init to scaffold application source. Application examples under `templates/examples/service/` are buildable fixtures, not application-source generators. Keep generated transport and persistence code owned by their generators.

## Boundaries

| Directory | Responsibility and dependency direction |
| --- | --- |
| `internal/domain` | Domain types, invariants and sentinels. Import only the explicitly permitted standard-library packages; no infrastructure, transport, persistence, logging or environment access. |
| `internal/application` | Use cases and their plain input and output. Depend on domain and permitted standard-library packages, never adapters or infrastructure. |
| `internal/adapters` | Inbound handlers and outbound repository or service implementations. Depend inward through ports; never import composition packages such as `cmd`, `app`, `wiring`, `router`, `middleware`, `telemetry` or `healthcheck`. |
| `internal/infrastructure` | Technical primitives for connections, HTTP error handling and instrumentation. Never import adapters or `cmd`. |
| `configs/` | Configuration parsing, defaults and validation. Environment reads stay here or at the entry point. |
| `cmd/` | Composition root: construct dependencies, connect adapters to ports and manage process lifetime. Reuse a repository's root main package when its layout selects it. |
| `db/queries` | SQL inputs for sqlc, only for a database context. Generated row types stay behind the repository boundary. |

An inbound port describes an application operation; its implementation coordinates domain behavior. An outbound port describes the persistence, clock or service behavior that operation needs. Declare each port in the package that calls it: an outbound port beside its use case in `internal/application`, an inbound port in the adapter that calls the use case. The iface linter of the kit's lint configuration reports an interface its own package never uses, so ports gathered in `internal/domain` or in a central `ports` package fail `make lint`. Keep contracts small and stated in domain terms. Concrete adapters satisfy them; the composition root chooses implementations. Domain and application code never discover infrastructure through globals. Any third-party import into these inner layers needs a decision record and an explicit policy allowance.

Propagate the incoming `context.Context` through `Execute`, ports and adapters to each call. Create process-root contexts at the entry point and test contexts with `t.Context()`. Honor cancellation, narrow deadlines at boundaries and release derived contexts. Never replace an incoming context with a fresh background context inside a use case.

Declare each package's failure modes once as sentinels in `errors.go`. Wrap an error crossing a package boundary with `package: action: %w`, preserving identity for `errors.Is`. Where the repository provides the established helper, begin the operation with `wrap := failures.Prefix("package: action")`; when three or more functions share a prefix, place their shared wrapper in `errors.go`. Never repeat the same layer in a chain, compare error strings, or persist `err.Error()` as business data. API adapters map sentinels through the single Problem Details boundary and keep internal chains out of responses.

## Shapes

### Domain and use cases

Describe the smallest domain values and invariants required by the operation, along with failure sentinels. Keep transport request types, database rows and SDK values out of these definitions. Validate external representations at the boundary and preserve domain invariants wherever a value is constructed.

Keep one use case per `verb_object.go` with its matching test file. Its shape is `New<Verb><Object>(deps)` and `Execute(ctx, input) (output, error)`: the constructor receives explicit ports, and `Execute` coordinates one operation. Use plain application input and output structs. Do not leak generated transport types, repository rows or transport status codes into them.

Inject a Clock port and compose UTC at the entry point. Domain and application code obtain time from that port, never from `time.Now`, `time.Since` or `time.Until`. Tests control the clock value instead of waiting for wall time.

### Adapters

Rows map to domain types in the repository. For a database context, place SQL in `db/queries`, call the generated sqlc methods and keep their row types inside the persistence adapter. The repository handles mapping and database error translation; the use case sees its port's values and sentinels.

Keep the generated strict handler signature. For an API context, the handler maps the generated request to application input, calls the inbound port, and maps output or a sentinel to the contract response. Change the API contract and regenerate before changing that boundary. Keep role enforcement, unknown-field rejection and error redaction explicit and testable.

When background jobs exist, give each job an explicit input, cancellation and shutdown path. Reuse the application operation through its port. Bound concurrency and retries, provide a parking path for exhausted work, and make permanent failure visible through liveness. Do not invent a scheduler or start background work in a constructor.

### Configuration and metrics

Each configurable value has an env tag, a default, validation and documentation. Keep its definition in `configs/` and document its meaning, units and allowed range beside it. Parse once, reject invalid configuration at startup, and pass typed values into constructors. Do not scatter environment reads or hidden defaults through the context.

A metric belongs to the package that emits it. Compose its instrumentation explicitly at startup, avoid global registration and bound label cardinality. Share one process version and propagate trace identifiers; never use credentials or unbounded user values as labels. The highest finite histogram bucket must cover the configured ceiling.

## Tests

- Default to black-box, parallel tests. Use `_internal_test.go` for a justified white-box test and the repository's declared dependencies and helpers. Keep each assertion tied to observable behavior.
- Test domain invariants and sentinel identities; test use cases with controlled ports and a deterministic Clock. Exercise cancellation, failure wrapping, duplicates and boundary values. Use `errors.Is` to assert the preserved cause.
- For adapters, verify row mapping, strict response shapes and error translation. For authenticated APIs, derive anonymous and unlisted-role cases from the contract. Give external-input parsers seeded fuzz targets and turn discovered failures into deterministic regression tests.
- Name integration files `*_integration_test.go` and guard them with `//go:build integration`. Keep the integration suite on demand and isolate its database or service dependencies. For growing tables, inspect realistic query plans; test simultaneous duplicate delivery and rollback rather than relying on sequential happy paths.
- Keep applied migrations immutable and expand before contracting. Test rollback to the empty starting state and reapplication, with data preservation and recovery checked in isolation.
- Every package that starts goroutines has a goroutine leak test using the repository's goleak dependency. Verify cancellation and shutdown paths so a passing operation does not leave background work alive.
- Measure coverage over the full build universe, not only `internal/` or changed packages. Require 80 percent total, per package and per file; the 20 statement minimum applies to package and file floors. Identify generated and mock exclusions explicitly, require all expected profile data and keep the kit free of exemptions.
- Use benchmarks for timing budgets, never functional wall-clock assertions. Mutation, fuzzing, benchmarks and integration remain on demand. Record their scope and any unmeasured result in the delivery evidence; remote CI or the platform pipeline is the only gate.

## Construction order

1. Read the policy and destination conventions, confirm the relevant capabilities, and define the operation's input universe, boundaries, failure modes and expected result. Follow the reuse ladder before adding a type or dependency.
2. Write and observe the failing behavioral test. Describe the domain values, sentinels and inbound and outbound ports needed to satisfy it, without transport or persistence details.
3. Implement the use case with constructor-injected ports, plain input and output, context propagation and a controlled Clock. Observe its tests passing before connecting infrastructure.
4. For an applicable database or API concern, change the query or contract inputs, regenerate owned code and implement the mapping adapters. Preserve generated signatures and prove the boundary and error paths.
5. Add configuration validation, package-owned metrics and any optional job lifecycle, then compose implementations and UTC time at the entry point. Check transaction and retry budgets before connecting external services.
6. Collect the relevant unit, race, coverage, lint, format and generated-freshness evidence, plus the on-demand results chosen for the change. Use the reviewer checklist and report commands, observed results and what remains unmeasured. No step requires a developer to arm local rungs.
