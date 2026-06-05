# Issue #15 Spec Review

Spec:

- `docs/superpowers/specs/2026-06-06-issue-15-catalog-near-cache-redis-design.md`

References:

- `/Users/debop/.codex/skills/bluetape4k-full-feature/references/step-2r-spec-review.md`
- `/Users/debop/.codex/skills/bluetape-go-patterns/SKILL.md`
- `docs/superpowers/research/2026-06-06-issue-15-catalog-near-cache-redis-research.md`

Execution mode:

- Local-equivalent review lanes. Native subagents were not spawned because the
  scope is one workshop example package and the main session can inspect the
  complete source evidence directly.

## Perspective Review

| Perspective | P0 | P1 | P2 | P3 | Notes |
|---|---:|---:|---:|---:|---|
| Developer / implementer | 0 | 0 | 0 | 0 | The accepted design composes existing `cache`, `redisnear`, and `rediscoord` APIs without adding reusable abstractions. |
| Security | 0 | 0 | 0 | 0 | Spec avoids durable Redis value cache claims and calls out ACL/TLS/namespace guidance for sensitive payloads in README scope. |
| Ops/SRE | 0 | 0 | 0 | 0 | Spec requires explicit peer close behavior and bounded eventual assertions for async Pub/Sub invalidation. |
| User / caller | 0 | 0 | 0 | 0 | README scope includes scenario, architecture, sequence/flow, run command, and operational boundaries. |

## 7-Tier Review

| Tier | Scope | P0 | P1 | P2 | P3 | Evidence |
|---|---|---:|---:|---:|---:|---|
| 1 Security | Redis payloads, namespaces, authoritative store | 0 | 0 | 0 | 0 | Spec keeps Redis as coordination/invalidation transport and requires operational boundary docs. |
| 2 Ops/SRE reliability | Pub/Sub async behavior, cleanup, client ownership | 0 | 0 | 0 | 0 | Spec states `Peer.Close` closes near-cache subscriber and Redis clients remain caller-owned. |
| 3 Structural impact | New example package only | 0 | 0 | 0 | 0 | No shared package, dependency, or workflow mutation is specified. |
| 4 Go API quality | Context, errors, narrow API, no Kotlin-shaped helpers | 0 | 0 | 0 | 0 | Spec keeps `Product`, `Store`, and `Peer` as narrow example-local types. |
| 5 Tests / silent failure | Invalidation, cold burst, missing SKU, race | 0 | 0 | 0 | 0 | Acceptance criteria include targeted test, race test, peer invalidation, reload, and one-loader proof. |
| 6 Performance / stability | Lock TTL, result TTL, polling, Testcontainers | 0 | 0 | 0 | 0 | Spec uses configurable short coordination timings and serial Testcontainers validation. |
| 7 Docs / release / evidence | README locale set, diagrams, root table, dependency drift | 0 | 0 | 0 | 0 | Spec requires EN/KO README, PNG embeds, SVG/Graphviz evidence, and no dependency changes. |

## Critic Integration

| Priority | Area | Finding | Resolution |
|---|---|---|---|
| None | - | No P0/P1/P2/P3 findings. | Spec is ready for Step 3. |

## Convergence

P0=0 P1=0

Step 2-R verdict: PASS.
