# Issue #68 Audited Order Workflow Outbox Lessons

## Context and Chosen Boundary

The workshop already had separate audit history and transactional outbox
lessons, while released `bluetape-go` v0.18.0 already supplied the audit
contracts, SQL outbox relay, and Redis Streams publisher. The useful next lesson
was their application boundary: accept one audited order command over HTTP,
commit current order state plus immutable history plus the outbox record in one
PostgreSQL transaction, and publish the committed audit event asynchronously.

History and outbox serve different truths. PostgreSQL order and history rows are
the durable query and audit source. The outbox is durable delivery intent. Redis
Streams is transport, not audit storage. Keeping those roles explicit prevents
Redis degradation from making committed workflow history unavailable and avoids
teaching a false distributed transaction.

## Replay, Readiness, and Shutdown

The idempotency key belongs to the whole command, not only to the outbox row. A
retry with the same identity and same canonical payload returns the stored
result. Reusing the identity with a different payload is a conflict scoped to
that command. The conflict path must not append a history revision or enqueue
another event.

Readiness follows durable ownership. SQL failure makes the service unready;
Redis transport failure is reported as degraded while accepted SQL work remains
recoverable by the relay. An unexpected relay exit still makes the process
unready because delivery progress has stopped. Status probes must remain
bounded so a failing dependency cannot consume the entire handler pool.

Cancellation alone is not a shutdown bound. The first lifecycle implementation
called relay cancellation and then waited indefinitely for relay join. A test
relay that ignored cancellation proved the advertised deadline could be
exceeded. The fix uses one shared deadline budget for HTTP shutdown and relay
join, forces server close when graceful HTTP shutdown spends the budget, and
returns a redacted timeout if the relay still does not join. Future supervised
workers should receive the same adversarial test before their lifecycle is
accepted.

## Integration and Tooling Surprises

Container-backed proof has to be serialized. PostgreSQL is brought up before
Redis, and the example integration, smoke, repository test, and race gates run
sequentially where they share Docker resources. Parallel green runs would not
prove deterministic ownership of ports, cleanup, or container lifecycle.

The first lint pass found 63 real quality findings: missing public teaching
documentation, unchecked row-close failures, direct error comparisons,
unwrapped errors, contextless network calls, staticcheck conversion and format
issues, and an empty compare-and-swap loop. Fixing them improved both the lesson
and failure semantics. Do not reduce a large lint count to a tooling problem
until every source finding is classified; use cache cleanup only for verified
stale-worktree paths.

The HTTP smoke test must execute the documented application, not merely call a
handler in process. That is what proves listener binding, JSON transport,
readiness, route wiring, lifecycle, and the copyable `requests.http` scenarios
describe the same program.

## Diagram Lessons

Automated geometry checks are necessary but incomplete. The final PNG, not only
the SVG source, must be inspected because marker rendering can reverse the
apparent direction or enlarge the arrowhead enough to collide with a bend. Give
every endpoint at least the marker-size terminal straight segment, end routes at
card edges, and move bend coordinates when the rendered marker consumes that
clearance.

The architecture audit checked all connectors, card intrusions, crossings,
endpoints, and mixed corners. The sequence audit additionally checked message
direction and activation layout. Original-pixel quadrant inspection was still
required: it caught a phase-title and pill overlap that the geometry scripts did
not flag. After moving the first message rows and activation bars, both the full
preview and all original-resolution crops were inspected again.

For future diagrams, retain this order:

1. audit connector counts, endpoints, crossings, card intrusions, and bend
   geometry in the SVG;
2. render PNG deterministically and compare hashes on a second render;
3. inspect every arrowhead direction and marker clearance in the PNG;
4. inspect original-pixel crops for text, pill, activation, and card overlap;
5. rerun every audit and eye inspection after any coordinate change.

## Future Guard

Do not turn this workshop into a generic workflow engine or promise exactly-once
delivery. Production adoption still requires authentication and authorization,
tenant isolation, TLS and proxy policy, schema migrations, outbox retention,
stream trimming, consumer groups, lag and dead-letter metrics, authorized replay
with audit records, and consumer deduplication by stable event identity.

Before extending the example, preserve these regression guards: atomic rollback,
same/different-payload replay, concurrent command serialization, relay lease
recovery, Redis degradation, unexpected relay exit, cancellation-ignoring relay
shutdown, actual-app HTTP smoke, bilingual contract parity, and rendered diagram
inspection.
