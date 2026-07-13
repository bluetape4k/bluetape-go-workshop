# Issue #57 Transactional Outbox Publisher Review

## Scope and Baseline

- Branch: `feat/issue-57-transactional-outbox-publisher`
- Base: `origin/develop@a7f2627d9e8457a4c88e910692a67016842a1032`
- Library baseline: released `bluetape-go` v0.18.0 APIs
- Scope: application-shaped PostgreSQL order placement plus the released SQL
  outbox relay and Redis Streams publisher; no reusable outbox implementation
- CodeGraph fallback: the repository query returned zero indexed nodes and
  edges, so source, tests, issue contract, and direct diff inspection were used

## Acceptance Review

| Contract | Evidence | Result |
| --- | --- | --- |
| Atomic order and outbox commit | Real PostgreSQL commit and rollback tests use one `sqlkit.WithTx` and `Store.Enqueue` | PASS |
| Released relay behavior | Success, exact 250 ms retry, third-attempt dead letter, cancellation, continuous run, and concurrent claim tests | PASS |
| Redis adapter contract | Real Redis test verifies 13 scalar/envelope fields and decoded `entry_json` parity | PASS |
| Stable delivery identity | Retry tests preserve `event_id` and `idempotency_key`; stale pending output fails closed | PASS |
| Resource lifecycle | Partial-open cleanup, idempotent close, joined cancellation, and close-failure stdout suppression tests | PASS |
| Runnable lesson | Pinned PostgreSQL/Redis commands produce the documented JSON and exactly 13 Redis fields | PASS |
| Bilingual documentation | English/Korean examples expose matched run, test, field, retry, replay, and production boundaries | PASS |
| Navigation | Both root READMEs link the example and its released package set | PASS |

## Six-Lens Review

| Lens | Result | Decision |
| --- | --- | --- |
| Performance | P0=0, P1=0 | Claims and stream verification are bounded; no throughput claim is made. |
| Stability | P0=0, P1=0 | Shared clocks, exact retry eligibility, joined cancellation, and deterministic cleanup are covered. |
| Security | P0=0, P1=0 | Identifiers are bounded/validated, SQL is parameterized, endpoints are not printed, and replay requires separate authorization/audit policy. |
| Operator/Ops | P0=0, P1=0 | SQL is named as the durable source; Redis is transport; recovery, retention, trimming, metrics, and replay automation remain explicit production work. |
| Developer/API | P0=0, P1=0 | Workshop code composes released store, relay, publisher, fixtures, and transaction helper without duplicating library abstractions. |
| User/caller | P0=0, P1=0 | Exact commands, output, rerun identities, failure boundaries, and both diagrams are visible in both locales. |

The independent code-review lane concluded `APPROVE` with no remaining
CRITICAL/HIGH/MEDIUM/LOW findings. The independent architecture lane concluded
`CLEAR` with P0/P1/P2/P3 all zero. Earlier review findings were repaired by:

- buffering success JSON until both clients close successfully;
- adding close-failure and stale-pending fail-closed regression tests;
- separating right-going `XADD` calls from left-going error/success returns;
- making the cancellation branch an independent claim-to-cancel sequence; and
- documenting the clean-state command, independent production lifecycles,
  durable SQL source, Redis transport role, and replay authorization boundary.

## Diagram Verification Ledger

| Asset | Automated evidence | Render and eye inspection |
| --- | --- | --- |
| Architecture | 4 markers, 8 connectors, 9 cards, 0 intrusions, 0 crossings, 0 geometry failures; endpoint and mixed-corner PASS with 3 Q bends | 3000x1800 PNG; SHA-256 `caecee685692d8269d11db6130dbc04c8ac7c568ed346bee6dbdca7704e46bf6`; CairoSVG byte parity PASS; original plus full-resolution quadrant inspection PASS |
| Sequence | 5 markers, 20 connectors, 6 cards, 0 intrusions, 0 crossings, 0 geometry failures; endpoint, mixed-corner, and sequence-style PASS | 3200x2440 PNG; SHA-256 `24903cf08a663364b2892c3f883219e1f7d65840d9cc1f704bac602780869394`; CairoSVG byte parity PASS; original plus full-resolution quadrant inspection PASS |

The PNG inspection explicitly checked arrowhead direction and size, dashed
return-marker rendering, terminal straight clearance before card edges, rounded
bends, activation endpoints, labels, line/card intrusion, crossings, and
whitespace. Calls point toward their receivers; error/success returns point back
to their callers.

## Fresh Validation

The following commands were observed to exit 0 after the last code and diagram
changes:

```bash
git diff --check
golangci-lint run ./examples/transactional-outbox-publisher/...
go test -count=1 ./examples/transactional-outbox-publisher/...
go test -race -count=1 ./examples/transactional-outbox-publisher/...
go test -count=10 ./examples/transactional-outbox-publisher/internal/orderoutbox -run '^TestRelayConcurrentRunOnce$'
make ci
```

The README container commands were also executed from a clean state with
`postgres:16-alpine` and `redis:7.4-alpine`. The command emitted the documented
newline-terminated JSON object, and `XRANGE` contained exactly the documented 13
field names. Both containers were removed afterward.

The first full lint attempt exposed missing exported comments, intentional nil
context literals, and a stale deleted-worktree cache entry. Source findings were
fixed, the cache was cleaned, and only the later focused lint plus fresh
`make ci` exit 0 are accepted as final evidence.

## Integration Decision

The implementation is PR-ready with P0=0 and P1=0. Delivery remains
at-least-once by design: an ambiguous Redis success can be published more than
once before the SQL row is marked. Consumers must deduplicate by stable event
identity. Merge, local synchronization, and worktree cleanup remain blocked on
new explicit user approval after live PR CI succeeds.
