# Issue #15 Catalog Near-Cache Redis Example Plan

Spec:

- `docs/superpowers/specs/2026-06-06-issue-15-catalog-near-cache-redis-design.md`

Research:

- `docs/superpowers/research/2026-06-06-issue-15-catalog-near-cache-redis-research.md`

## Constraints

- Apply `$bluetape-go-patterns` to every Go implementation and test task.
- Apply `$bluetape4k-diagram` to README diagram assets.
- Keep the example thin and scenario-first.
- Reuse `redistestcontainer.Start(ctx,t)`; do not add container helpers.
- Do not add dependencies.
- Run Testcontainers-backed tests serially.
- Commit spec and plan before implementation.

## Tasks

### T0. Run Step 3-P Pre-Implementation Prediction

- complexity: medium
- References:
  - `/Users/debop/.codex/skills/bluetape4k-full-feature/references/step-4p-perf-scan.md`
- Work:
  - Predict cache, Pub/Sub, Redis lock, goroutine lifecycle, Testcontainers,
    and diagram-evidence risks before implementation.
  - Feed mitigations into T1-T6 before editing Go code.
- Verification:
  - Step 3-P notes recorded in progress and review evidence before Step 4.

### T1. Create Example Domain Package

- complexity: medium
- Files:
  - `examples/catalog-near-cache-redis/internal/catalogcache/catalog.go`
- Apply:
  - `$bluetape-go-patterns`
- Work:
  - Add `Product`, `Store`, `Peer`, `Options`.
  - Keep `Store` concurrency-safe and example-local.
  - Keep Redis clients caller-owned.
  - Implement `NewPeer`, `GetProduct`, `PutProduct`, and `Close`.
  - Compose `cache.NewMemory`, `redisnear.NewPubSub`, and
    `rediscoord.NewStampedeCache`.
- Recheck before implementation:
  - `redisnear.Options` and `rediscoord.Options` signatures from current
    `go list -m -f '{{.Dir}}' github.com/bluetape4k/bluetape-go`.
- Verification:
  - `gofmt -w examples/catalog-near-cache-redis/internal/catalogcache/catalog.go`
  - `go test -count=1 ./examples/catalog-near-cache-redis/...`

### T2. Add Peer Invalidation Integration Tests

- complexity: high
- Files:
  - `examples/catalog-near-cache-redis/internal/catalogcache/catalog_test.go`
- Apply:
  - `$bluetape-go-patterns`
- Work:
  - Start Redis through `redistestcontainer.Start(ctx,t)`.
  - Create peer A and peer B with distinct Redis clients and `OriginID`s.
  - Prime peer B with stale authoritative value.
  - Write a newer product through peer A.
  - Prove peer B's local cache is invalidated eventually.
  - Prove peer B reloads the authoritative product on next read.
  - Close peers and Redis clients through `t.Cleanup`.
- Verification:
  - `go test -count=1 ./examples/catalog-near-cache-redis/...`

### T3. Add Cold-Miss Stampede Coordination Tests

- complexity: high
- Files:
  - `examples/catalog-near-cache-redis/internal/catalogcache/catalog_test.go`
- Apply:
  - `$bluetape-go-patterns`
- Work:
  - Create two cold peers for the same namespace.
  - Use a blocking store loader or hook to hold the owner load.
  - Run concurrent `GetProduct` calls from both peers for the same SKU.
  - Wait until exactly one loader invocation is observed, release it, then
    assert both peers receive the same product.
  - Assert loader count remains one.
  - Add missing SKU and close/idempotency coverage if implementation exposes
    those paths.
- Verification:
  - `go test -count=1 ./examples/catalog-near-cache-redis/...`
  - `go test -race -count=1 ./examples/catalog-near-cache-redis/...`

### T4. Add Example README Locale Set

- complexity: medium
- Files:
  - `examples/catalog-near-cache-redis/README.md`
  - `examples/catalog-near-cache-redis/README.ko.md`
  - `README.md`
  - `README.ko.md`
- Apply:
  - `$bluetape-go-patterns` for Go snippets and commands.
  - `$bluetape4k-diagram` for diagram embeds.
- Work:
  - Use `[English](README.md) | [한국어](README.ko.md)`.
  - Explain scenario, architecture, cache flow, sequence, operational
    boundaries, outcomes, and run command.
  - Embed PNG diagrams only.
  - Update root README example tables with `English | 한국어` links.
- Verification:
  - `rg -n "catalog-near-cache-redis|English \\| 한국어|\\.png" README.md README.ko.md examples/catalog-near-cache-redis`
  - `git diff --check`

### T5. Generate README Diagrams

- complexity: high
- Files:
  - `docs/images/readme-diagrams/catalog-near-cache-redis-scenario.*`
  - `docs/images/readme-diagrams/catalog-near-cache-redis-architecture.*`
  - `docs/images/readme-diagrams/catalog-near-cache-redis-sequence.*`
- Apply:
  - `$bluetape4k-diagram`
- Work:
  - Create Graphviz `.dot`, `.plain`, `*-graphviz.svg`,
    `*-graphviz.png` evidence for each node-and-connector diagram.
  - Create final SVG and PNG assets.
  - Use English labels in generated images.
  - Use `Architects Daughter` for title/prominent labels and `Comic Mono` for
    details/captions.
  - Print deterministic geometry gate summaries with:
    `nodes`, `routes`, `segments`, `badEndpointAngle`, `badBends`,
    `interiorCrossings`, `marginImbalance`, `titleGap`.
  - Inspect each rendered PNG individually.
- Verification:
  - `dot -Tplain ...`
  - `dot -Tsvg ...`
  - `dot -Tpng ...`
  - `rsvg-convert ...`
  - `view_image` inspection for every final PNG.

### T6. Run Targeted And Repository Validation

- complexity: medium
- Apply:
  - `$bluetape-go-patterns`
- Commands:
  - `go test -count=1 ./examples/catalog-near-cache-redis/...`
  - `go test -race -count=1 ./examples/catalog-near-cache-redis/...`
  - `go test ./...`
  - `make ci`
  - `git diff --check`
- Notes:
  - Keep Testcontainers-backed commands serial.
  - If a full command fails from an unrelated environment issue, record the
    exact failing package and rerun the narrowest command that proves this
    example.

### T7. Step 5 Verifier Checklist

- complexity: medium
- References:
  - `/Users/debop/.codex/skills/bluetape4k-full-feature/references/step-5-verifier-checklist.md`
- Work:
  - Map spec acceptance criteria and plan tasks to implementation, tests,
    README, diagram evidence, validation, and dependency state.
- Verification:
  - Produce PASS/FAIL verifier notes in Step DoD evidence.

### T8. Step 6-R 7-Tier Code Review

- complexity: high
- References:
  - `/Users/debop/.codex/skills/bluetape4k-full-feature/references/step-6r-code-review.md`
  - `/Users/debop/.codex/skills/bluetape4k-full-feature/references/step-4p-perf-scan.md`
- Work:
  - Review the changed example slice across tiers 1-7.
  - Apply `bluetape-go-patterns` for Go API/context/error/concurrency/test
    checks.
  - Record P0/P1/P2/P3 counts and convergence.
- Verification:
  - P0=0 and P1=0 before Step 7.
  - Save tracked review artifact.

### T9. Lessons, Commit, PR, PR Review, CI

- complexity: medium
- Files:
  - `docs/lessons/2026-06-06-catalog-near-cache-redis.md`
  - PR body temp file
- Work:
  - Commit spec and plan before implementation.
  - Commit implementation/docs/diagrams/lessons with Lore messages.
  - Push branch and create PR against `develop`.
  - PR body final section must be `## DoD Status`.
  - Run Step 7-R PR review and leave PR review/comment evidence.
  - Wait for GitHub CI.
- Verification:
  - `gh pr view <number> --json body` final `##` heading is `## DoD Status`.
  - `gh pr checks <number>` all required checks pass or blocker recorded.

## Acceptance Mapping

| Acceptance | Plan Task |
|---|---|
| Targeted example test passes | T2, T3, T6 |
| Peer A write invalidates peer B local cache | T1, T2 |
| Peer B reloads authoritative product | T1, T2 |
| Cold miss burst runs loader once | T1, T3 |
| Uses repository Redis fixture | T2, T3 |
| No new dependencies | T1, T6, T8 |
| README and root tables updated | T4 |
| Rich diagrams added and verified | T5 |

## Rollback Points

- If `redisnear` invalidation timing is unstable, keep implementation unchanged
  and adjust tests to assert eventual miss/reload with bounded polling.
- If stampede coordination cannot prove exactly one load reliably, stop before
  PR and revisit the cold burst harness; do not weaken the acceptance criterion.
- If diagram gates fail, enlarge canvas or reroute connectors before README
  embedding.
