# Issue #15 명세 리뷰

명세:

- `docs/superpowers/specs/2026-06-06-issue-15-catalog-near-cache-redis-design.md`

참조:

- `/Users/debop/.codex/skills/bluetape4k-full-feature/references/step-2r-spec-review.md`
- `/Users/debop/.codex/skills/bluetape-go-patterns/SKILL.md`
- `docs/superpowers/research/2026-06-06-issue-15-catalog-near-cache-redis-research.md`

실행 모드:

- local-equivalent review lane. scope가 하나의 workshop example package이고 main
  session에서 전체 source evidence를 직접 검사할 수 있어 native subagent를 띄우지 않았다.

## 관점 리뷰

| Perspective | P0 | P1 | P2 | P3 | Notes |
|---|---:|---:|---:|---:|---|
| Developer / implementer | 0 | 0 | 0 | 0 | 채택한 design은 reusable abstraction을 추가하지 않고 기존 `cache`, `redisnear`, `rediscoord` API를 조합한다. |
| Security | 0 | 0 | 0 | 0 | spec은 durable Redis value cache claim을 피하고, README scope에서 sensitive payload에 대한 ACL/TLS/namespace guidance를 명시한다. |
| Ops/SRE | 0 | 0 | 0 | 0 | spec은 explicit peer close behavior와 async Pub/Sub invalidation을 위한 bounded eventual assertion을 요구한다. |
| User / caller | 0 | 0 | 0 | 0 | README scope에는 scenario, architecture, sequence/flow, run command, operational boundary가 포함된다. |

## 7-Tier 리뷰

| Tier | Scope | P0 | P1 | P2 | P3 | Evidence |
|---|---|---:|---:|---:|---:|---|
| 1 Security | Redis payload, namespace, authoritative store | 0 | 0 | 0 | 0 | spec은 Redis를 coordination/invalidation transport로 유지하고 operational boundary docs를 요구한다. |
| 2 Ops/SRE reliability | Pub/Sub async behavior, cleanup, client ownership | 0 | 0 | 0 | 0 | spec은 `Peer.Close`가 near-cache subscriber를 닫고 Redis client는 caller-owned로 남는다고 명시한다. |
| 3 Structural impact | 새 example package only | 0 | 0 | 0 | 0 | shared package, dependency, workflow mutation은 명시되지 않았다. |
| 4 Go API quality | context, error, narrow API, no Kotlin-shaped helper | 0 | 0 | 0 | 0 | spec은 `Product`, `Store`, `Peer`를 narrow example-local type으로 유지한다. |
| 5 Tests / silent failure | invalidation, cold burst, missing SKU, race | 0 | 0 | 0 | 0 | acceptance criteria는 targeted test, race test, peer invalidation, reload, one-loader proof를 포함한다. |
| 6 Performance / stability | lock TTL, result TTL, polling, Testcontainers | 0 | 0 | 0 | 0 | spec은 configurable short coordination timing과 serial Testcontainers validation을 사용한다. |
| 7 Docs / release / evidence | README locale set, diagram, root table, dependency drift | 0 | 0 | 0 | 0 | spec은 EN/KO README, PNG embed, SVG/Graphviz evidence, dependency change 금지를 요구한다. |

## Critic Integration

| Priority | Area | Finding | Resolution |
|---|---|---|---|
| None | - | P0/P1/P2/P3 finding이 없다. | spec은 Step 3 준비가 되었다. |

## 수렴

P0=0 P1=0

Step 2-R 판정: PASS.
