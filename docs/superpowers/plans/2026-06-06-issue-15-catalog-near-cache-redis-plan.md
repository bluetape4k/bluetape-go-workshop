# Issue #15 Catalog Near-Cache Redis 예제 계획

Spec:

- `docs/superpowers/specs/2026-06-06-issue-15-catalog-near-cache-redis-design.md`

Research:

- `docs/superpowers/research/2026-06-06-issue-15-catalog-near-cache-redis-research.md`

## 제약

- 모든 Go implementation 및 test task에 `$bluetape-go-patterns`를 적용한다.
- README diagram asset에는 `$bluetape4k-diagram`을 적용한다.
- 예제는 thin하고 scenario-first로 유지한다.
- `redistestcontainer.Start(ctx,t)`를 재사용한다. container helper를 추가하지 않는다.
- dependency를 추가하지 않는다.
- Testcontainers-backed test는 직렬로 실행한다.
- 구현 전에 spec과 plan을 commit한다.

## 작업

### T0. Step 3-P 사전 구현 예측 실행

- complexity: medium
- References:
  - `/Users/debop/.codex/skills/bluetape4k-full-feature/references/step-4p-perf-scan.md`
- Work:
  - 구현 전에 cache, Pub/Sub, Redis lock, goroutine lifecycle, Testcontainers,
    diagram-evidence risk를 예측한다.
  - Go code를 편집하기 전에 mitigation을 T1-T6에 반영한다.
- Verification:
  - Step 4 전에 Step 3-P note를 progress 및 review evidence에 기록한다.

### T1. 예제 도메인 패키지 생성

- complexity: medium
- Files:
  - `examples/catalog-near-cache-redis/internal/catalogcache/catalog.go`
- Apply:
  - `$bluetape-go-patterns`
- Work:
  - `Product`, `Store`, `Peer`, `Options`를 추가한다.
  - `Store`를 concurrency-safe 및 example-local로 유지한다.
  - Redis client는 caller-owned로 유지한다.
  - `NewPeer`, `GetProduct`, `PutProduct`, `Close`를 구현한다.
  - `cache.NewMemory`, `redisnear.NewPubSub`,
    `rediscoord.NewStampedeCache`.
- 구현 전 재확인:
  - 현재 `go list -m -f '{{.Dir}}' github.com/bluetape4k/bluetape-go` 기준
    `redisnear.Options` 및 `rediscoord.Options` signature.
- Verification:
  - `gofmt -w examples/catalog-near-cache-redis/internal/catalogcache/catalog.go`
  - `go test -count=1 ./examples/catalog-near-cache-redis/...`

### T2. Peer invalidation integration test 추가

- complexity: high
- Files:
  - `examples/catalog-near-cache-redis/internal/catalogcache/catalog_test.go`
- Apply:
  - `$bluetape-go-patterns`
- Work:
  - `redistestcontainer.Start(ctx,t)`로 Redis를 시작한다.
  - 서로 다른 Redis client와 `OriginID`를 가진 peer A 및 peer B를 만든다.
  - stale authoritative value로 peer B를 prime한다.
  - peer A를 통해 더 새로운 product를 쓴다.
  - peer B의 local cache가 eventually invalidated됨을 증명한다.
  - 다음 read에서 peer B가 authoritative product를 reload함을 증명한다.
  - `t.Cleanup`으로 peer와 Redis client를 닫는다.
- Verification:
  - `go test -count=1 ./examples/catalog-near-cache-redis/...`

### T3. Cold-miss stampede coordination test 추가

- complexity: high
- Files:
  - `examples/catalog-near-cache-redis/internal/catalogcache/catalog_test.go`
- Apply:
  - `$bluetape-go-patterns`
- Work:
  - 같은 namespace에 대해 cold peer 두 개를 만든다.
  - blocking store loader 또는 hook으로 owner load를 붙잡는다.
  - 같은 SKU에 대해 두 peer에서 concurrent `GetProduct` call을 실행한다.
  - 정확히 하나의 loader invocation이 관찰될 때까지 기다린 뒤 release하고, 두 peer가
    같은 product를 받는지 assert한다.
  - loader count가 하나로 유지되는지 assert한다.
  - implementation이 해당 path를 노출하면 missing SKU와 close/idempotency coverage를
    추가한다.
- Verification:
  - `go test -count=1 ./examples/catalog-near-cache-redis/...`
  - `go test -race -count=1 ./examples/catalog-near-cache-redis/...`

### T4. 예제 README locale set 추가

- complexity: medium
- Files:
  - `examples/catalog-near-cache-redis/README.md`
  - `examples/catalog-near-cache-redis/README.ko.md`
  - `README.md`
  - `README.ko.md`
- Apply:
  - Go snippet과 command에는 `$bluetape-go-patterns`.
  - diagram embed에는 `$bluetape4k-diagram`.
- Work:
  - `[English](README.md) | [한국어](README.ko.md)`를 사용한다.
  - scenario, architecture, cache flow, sequence, operational boundary, outcome,
    run command를 설명한다.
  - PNG diagram만 embed한다.
  - root README example table을 `English | 한국어` link로 갱신한다.
- Verification:
  - `rg -n "catalog-near-cache-redis|English \\| 한국어|\\.png" README.md README.ko.md examples/catalog-near-cache-redis`
  - `git diff --check`

### T5. README 다이어그램 생성

- complexity: high
- Files:
  - `docs/images/readme-diagrams/catalog-near-cache-redis-scenario.*`
  - `docs/images/readme-diagrams/catalog-near-cache-redis-architecture.*`
  - `docs/images/readme-diagrams/catalog-near-cache-redis-sequence.*`
- Apply:
  - `$bluetape4k-diagram`
- Work:
  - 각 node-and-connector diagram에 대해 Graphviz `.dot`, `.plain`,
    `*-graphviz.svg`, `*-graphviz.png` evidence를 만든다.
  - 최종 SVG 및 PNG asset을 만든다.
  - generated image에는 English label을 사용한다.
  - title/prominent label에는 `Architects Daughter`, detail/caption에는
    `Comic Mono`를 사용한다.
  - 다음 항목으로 deterministic geometry gate summary를 출력한다:
    `nodes`, `routes`, `segments`, `badEndpointAngle`, `badBends`,
    `interiorCrossings`, `marginImbalance`, `titleGap`.
  - 각 rendered PNG를 개별 점검한다.
- Verification:
  - `dot -Tplain ...`
  - `dot -Tsvg ...`
  - `dot -Tpng ...`
  - `rsvg-convert ...`
  - 모든 final PNG에 대한 `view_image` inspection.

### T6. Targeted 및 repository 검증 실행

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
  - Testcontainers-backed command는 serial로 유지한다.
  - full command가 unrelated environment issue로 실패하면 정확한 failing package를
    기록하고 이 example을 증명하는 가장 좁은 command를 다시 실행한다.

### T7. Step 5 검증기 체크리스트

- complexity: medium
- References:
  - `/Users/debop/.codex/skills/bluetape4k-full-feature/references/step-5-verifier-checklist.md`
- Work:
  - spec acceptance criteria와 plan task를 implementation, test, README,
    diagram evidence, validation, dependency state에 매핑한다.
- Verification:
  - Step DoD evidence에 PASS/FAIL verifier note를 만든다.

### T8. Step 6-R 7-tier code review

- complexity: high
- References:
  - `/Users/debop/.codex/skills/bluetape4k-full-feature/references/step-6r-code-review.md`
  - `/Users/debop/.codex/skills/bluetape4k-full-feature/references/step-4p-perf-scan.md`
- Work:
  - 변경된 example slice를 tier 1-7 전체에서 review한다.
  - Go API/context/error/concurrency/test check에는 `bluetape-go-patterns`를
    적용한다.
  - P0/P1/P2/P3 count와 convergence를 기록한다.
- Verification:
  - Step 7 전에 P0=0 및 P1=0.
  - tracked review artifact를 저장한다.

### T9. Lesson, commit, PR, PR review, CI

- complexity: medium
- Files:
  - `docs/lessons/2026-06-06-catalog-near-cache-redis.md`
  - PR body temp file
- Work:
  - 구현 전에 spec과 plan을 commit한다.
  - Lore message로 implementation/docs/diagrams/lessons를 commit한다.
  - branch를 push하고 `develop` 대상 PR을 만든다.
  - PR body final section은 반드시 `## DoD Status`여야 한다.
  - Step 7-R PR review를 실행하고 PR review/comment evidence를 남긴다.
  - GitHub CI를 기다린다.
- Verification:
  - `gh pr view <number> --json body`의 final `##` heading이 `## DoD Status`.
  - `gh pr checks <number>`에서 모든 required check가 pass하거나 blocker가 기록됨.

## 인수 조건 매핑

| 인수 조건 | Plan task |
|---|---|
| targeted example test 통과 | T2, T3, T6 |
| peer A write가 peer B local cache를 invalidate | T1, T2 |
| peer B가 authoritative product를 reload | T1, T2 |
| cold miss burst가 loader를 한 번만 실행 | T1, T3 |
| repository Redis fixture 사용 | T2, T3 |
| 새 dependency 없음 | T1, T6, T8 |
| README 및 root table 갱신 | T4 |
| 풍부한 diagram 추가 및 검증 | T5 |

## Rollback point

- `redisnear` invalidation timing이 불안정하면 implementation은 그대로 두고 test를
  bounded polling으로 eventual miss/reload를 assert하도록 조정한다.
- stampede coordination이 정확히 한 번의 load를 안정적으로 증명할 수 없으면 PR
  전에 중단하고 cold burst harness를 재검토한다. acceptance criterion을 약화하지
  않는다.
- diagram gate가 실패하면 README embedding 전에 canvas를 키우거나 connector를
  reroute한다.
