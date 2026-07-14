# Issue #68 Audited Order Workflow Outbox Review

## Scope and Baseline

- Branch: `feat/issue-68-audited-order-workflow-outbox`
- Base: `origin/develop@64d1a80dd88ecc365957b19f7fb74d70098120fc`
- Reviewed implementation head: `b453cc4`
- Library baseline: released `bluetape-go` v0.18.0 APIs
- Scope: an application-shaped HTTP order workflow that commits the order,
  immutable audit history, and SQL outbox record atomically, then relays the
  audit event to Redis Streams
- Boundary: no reusable history store, outbox relay, or Redis publisher was
  reimplemented in the workshop

## Acceptance Review

| Contract | Evidence | Result |
| --- | --- | --- |
| Atomic workflow write | Real PostgreSQL tests commit order state, immutable audit history, and the official SQL outbox entry through one `*sql.Tx`; rollback leaves none of them behind | PASS |
| Strict POST JSON API | The handler enforces JSON content type, one bounded object, known fields, unique keys, valid UTF-8, and bounded identity, reason, and metadata values | PASS |
| Replay semantics | Same identity and same payload replay the stored response; same identity with a different payload returns a scoped conflict without duplicating history or outbox data | PASS |
| History query | The store implements the released audit history reader contract with bounded cursor pagination and immutable ordered revisions | PASS |
| Released relay integration | A supervised `sqloutbox.Relay` publishes through the released Redis Streams adapter, preserves event identity, recovers leases, and tolerates Redis degradation without losing the SQL source | PASS |
| Lifecycle and readiness | SQL is a hard readiness dependency, Redis/relay state is visible separately, unexpected relay exit fails readiness, and shutdown shares one hard deadline | PASS |
| Runnable lesson | `curl` and `requests.http` provide POST JSON scenarios for success, replay, conflict, validation, history, readiness, and status inspection | PASS |
| Bilingual documentation | English and Korean READMEs describe the same run path, API contract, architecture, failure model, and production boundaries | PASS |
| Visual explanation | Matching SVG/PNG architecture and sequence diagrams are embedded in both locales and passed automated plus rendered visual checks | PASS |

## Six-Lens Review

| Lens | Result | Decision |
| --- | --- | --- |
| Performance | P0=0, P1=0 | Request bodies, concurrency, database and Redis pools, query pages, relay claims, and status probes are bounded; no throughput claim is made. |
| Stability | P0=0, P1=0 | Row locking, transaction rollback, concurrent replay/conflict tests, lease recovery, relay supervision, restart/backlog tests, and one shared shutdown deadline cover the failure boundaries. |
| Security | P0=0, P1=0 | The example binds only to a loopback literal, disables trusted proxies, validates and bounds input, parameterizes SQL, rejects compression, and redacts public errors and logs. Authentication, TLS, authorization, and tenant isolation remain explicit production work. |
| Operator/Ops | P0=0, P1=0 | Health, readiness, and bounded status endpoints separate durable SQL health from Redis transport degradation and relay state. The runbook documents reset, retention, replay, and at-least-once boundaries. |
| Developer/API | P0=0, P1=0 | Workshop code composes released v0.18.0 audit, SQL outbox, Redis Streams, and Testcontainers APIs without adding a dependency or changing shared workflows. Exported teaching APIs have useful documentation and lint is clean. |
| User/caller | P0=0, P1=0 | Copyable POST JSON examples include metadata, exact response and error behavior, replay identity, cursor pagination, readiness, and status inspection in both locales. |

The integrated review has no remaining P0, P1, P2, or P3 finding. One P1 was
found during review: lifecycle shutdown waited without a bound for a relay that
ignored cancellation, so it could exceed the advertised hard shutdown
deadline. A failing regression first reproduced `relay join exceeded shutdown
deadline`; the runtime now spends one shared deadline across HTTP shutdown and
relay join, forces server close when necessary, returns the redacted
`relay_shutdown: timeout` failure, and marks readiness false.

The first repository lint run reported 63 findings rather than being treated as
noise. The fixes added package and exported-symbol documentation, propagated
`rows.Close` errors, used `errors.Is` and wrapped errors, replaced contextless
network operations, corrected staticcheck conversions and formatting, and made
the compare-and-swap loop termination explicit. The final lint run reported
`0 issues` without suppressions.

## Diagram Verification Ledger

| Asset | Automated evidence | Render and eye inspection |
| --- | --- | --- |
| Architecture | 5 markers, 10 connectors, 11 cards, 0 intrusions, 0 crossings, 0 geometry failures; endpoint and mixed-corner PASS with 10 quadratic bends | 3200x2000 PNG; SHA-256 `91331f16a2efab3b0ab6ed6ee6e2b2cc94a3a1ec7743978fe71cb5d069c510a2`; deterministic rerender PASS; full source-size preview and four original-pixel quadrant inspections PASS |
| Sequence | 6 markers, 22 connectors, 22 numbered pills and circular badges, 6 cards, 0 intrusions, 0 crossings, 0 geometry failures; endpoint, mixed-corner, and sequence-style PASS | 3200x3650 PNG; SHA-256 `e2f1dc14ddcc60c6222d24e06ce715d3759c565245dbb5cb53018087c3f91d08`; deterministic rerender PASS; full source-size preview and four original-pixel quadrant inspections PASS |

The rendered PNG inspection explicitly checked every arrowhead after SVG to
PNG conversion, terminal straight clearance relative to marker size, rounded
bend placement, receiver direction, label/card overlap, route merging,
clipping, and whitespace. The sequence diagram initially had a phase-title and
pill overlap that automated geometry checks did not report. Moving the first
message rows and activation bars fixed it; the final render and pixel crops were
then inspected again. A later best-practices comparison also found that the
plain inline call numbers used an older label style. All 22 labels now use the
approved 34-pixel pill with a semantic-color 13-pixel circular number badge,
10-pixel label-to-line clearance, and at least 6 pixels between adjacent rows.

## Fresh Validation

The following commands were observed to exit 0 after the final implementation
changes:

```bash
go test -count=1 ./examples/audited-order-workflow-outbox/...
go test -race -count=1 ./examples/audited-order-workflow-outbox/...
go test -count=1 -p 1 ./examples/audited-order-workflow-outbox -run '^TestRequestsHTTPSmoke$'
go test -p 1 -count=1 ./...
make fmt-check
make tidy-check
make vet
make lint
make test
make race
make ci
git diff --check origin/develop
```

The Testcontainers-backed checks ran sequentially. The smoke test started the
actual application and exercised the documented HTTP request contract rather
than calling handlers directly. Lost process handles and earlier partial
outputs were not accepted as evidence; the recorded full gates have observed
exit code 0.

## Verifier Decision

| Gate | Result |
| --- | --- |
| Approved scope and released baseline | PASS |
| Contract and failure-path tests | PASS |
| Bilingual README parity and navigation | PASS |
| Diagram source, render, audits, and eye inspection | PASS |
| Repository formatting, tidy, vet, lint, normal tests, and race tests | PASS |
| Six-lens review with P0=0 and P1=0 | PASS |
| Clean integration handoff | PASS |

Required checks: 7/7; N/A: 0; Blocked: 0.

## Integration Decision

The implementation is locally PR-ready. Delivery remains at-least-once: Redis
may accept an event before the relay can durably mark the SQL outbox record, so
consumers must deduplicate by stable event identity. Push and PR creation are a
separate handoff step. Merge, local synchronization, and worktree cleanup remain
blocked on explicit user approval after live PR CI succeeds and review threads
are rechecked.
