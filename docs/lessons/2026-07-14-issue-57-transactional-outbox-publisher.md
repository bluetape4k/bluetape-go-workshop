# Issue #57 Transactional Outbox Publisher Lessons

## Context and Boundary

The workshop already had a service-owned SQL transaction example, while
bluetape-go v0.18.0 already provided `audit/sqloutbox`, its deterministic test
publisher, and the Redis Streams adapter. The useful lesson was therefore not a
new outbox implementation. It was the application boundary: insert the order
and call `Store.Enqueue` through the same `*sql.Tx`, return the order only after
commit, and let a caller-owned relay publish later.

The runnable command deliberately executes one bounded `RunOnce` batch. A
pre-existing pending record wins normal outbox ordering, so the command compares
the published identity with the new command and fails closed instead of printing
the wrong event as success. A production worker needs a continuous relay and an
explicit restart/replay policy rather than this single-record teaching invariant.

Success output is also part of the lifecycle contract. The command buffers its
JSON until PostgreSQL and Redis clients close successfully, so a shutdown error
cannot leave a plausible success object on stdout beside a nonzero exit. A
dedicated close-failure test asserts that stdout remains empty.

## Retry, Identity, and Cancellation

Store and relay tests must share the same injected clock. Advancing only one
clock can produce a misleading retry test because the persisted `retry_at` and
the claim eligibility check no longer describe the same timeline. The test now
proves no claim before 250 ms, eligibility exactly at 250 ms, attempts 1 and 2,
and unchanged `event_id` and `idempotency_key`.

Caller cancellation at the publisher boundary is not an ordinary delivery
failure. The released relay returns `context.Canceled` and leaves the record
claimed at attempt 1; it does not schedule a retry or dead-letter the row.
Continuous-run shutdown therefore needs a joined goroutine test, not a sleep or
an assertion that cancellation rewrites delivery state.

At-least-once remains a consumer contract. Redis can accept `XADD` before the
publisher observes an ambiguous error or before the SQL completion mark becomes
durable. Retries and operator replay must preserve identity, and consumers must
deduplicate rather than treating the attempt counter as identity.

## Integration and Tooling Misses

The real PostgreSQL/Redis test checks all 13 Redis fields, decodes `entry_json`,
and compares its aggregate, revision, event identity, event type, and schema
version with the scalar fields. Merely checking stream length would have proved
transport reachability but not adapter compatibility.

The first repository-wide lint run found two independent causes:

- new exported teaching symbols lacked the repository's required package and
  API comments, and literal nil contexts in contract tests triggered SA1012;
- golangci-lint still held findings for a deleted worktree.

Useful English comments fixed the source findings. Typed zero-value
`context.Context` variables keep the intentional nil-context contract visible
without suppressing static analysis. A focused package lint then proved the
source fix; `golangci-lint cache clean` followed by repository-wide lint proved
the deleted path was environmental. The full `make ci` was rerun from the
beginning and accepted only after its observed exit code was 0.

## Diagram Lessons

An SVG path is not part of the connector audit merely because it has a marker.
The first sequence source used semantic classes such as `callBlue` but omitted
the common `connector` class, so only two message paths were counted. Every
message path now carries both classes, and the final audit reports all 20.

SVG correctness is also insufficient evidence for a PNG deliverable. The final
inspection rendered both diagrams with CairoSVG, checked deterministic PNG
parity, and opened original-resolution quadrant crops without resizing. That
caught or guarded against the failures that matter after conversion:

- arrowheads pointing opposite to the intended message direction;
- bends placed too close to the arrowhead, leaving no terminal straight segment;
- multiple semantic routes sharing a card port and visually merging;
- labels overflowing cards or using unsupported glyphs; and
- a whole-image viewer artifact hiding otherwise valid pixels.

The architecture separates transaction and relay ports on the outbox card and
keeps at least the marker-size clearance before every terminal. The sequence
uses explicit fixed-size markers, left-pointing return arrows, right-pointing
calls, and distinct cancellation color. Automated connector, geometry,
endpoint, mixed-corner, and sequence-style audits supplement but do not replace
that PNG eye inspection.

Sequence semantics need the same scrutiny as geometry. `XADD` is a right-going
publisher-to-Redis call; the error or success is a separate dashed left-going
return. Drawing only “XADD returns error” toward Redis can have a technically
correct arrowhead while teaching the opposite interaction. The cancellation
branch likewise starts its own claim before showing publish cancellation, so it
does not visually depend on the success branch above it.

## Future Guard

Do not extend this example into a reusable worker framework. Production work
belongs in application infrastructure or bluetape-go and needs measured batch
size, connection pooling, consumer groups, stream trimming, outbox retention,
lag metrics, dead-letter inspection and authorized replay, tenant isolation,
authentication/TLS, and versioned migrations. Reopen the design before adding
automated replay or exactly-once claims; both change the operational contract.
