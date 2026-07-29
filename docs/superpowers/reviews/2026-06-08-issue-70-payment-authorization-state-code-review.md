# Issue #70 코드 리뷰

## 판정

- Gate: PASS
- P0: 0
- P1: 0
- reviewer stance: bluetape-go P0/P1 rule을 적용한 Step 6-R implementation review.

## 검토 범위

- `examples/payment-authorization-state/main.go`
- `examples/payment-authorization-state/internal/paymentauth/server.go`
- `examples/payment-authorization-state/internal/paymentauth/server_test.go`
- `examples/payment-authorization-state/README.md`
- `examples/payment-authorization-state/README.ko.md`
- `docs/images/readme-diagrams/payment-authorization-state-*`
- `docs/images/readme-diagrams/workshop-example-map.*`
- `scripts/generate-payment-authorization-state-diagrams.sh`

## Tier finding

| Tier | 결과 | P0 | P1 | 근거 |
|---|---:|---:|---:|---|
| Security | PASS | 0 | 0 | in-memory example에는 auth boundary, external storage, command execution이 없다. JSON input은 `server.go:290-317`의 event parsing으로 제한된다. |
| Ops/SRE reliability | PASS | 0 | 0 | `/healthz`는 `server.go:180-182`에 있다. stable error code는 `server.go:320-340`에서 매핑된다. README는 production storage/audit gap을 문서화한다. |
| Structural impact | PASS | 0 | 0 | 새 isolated example package만 추가된다. state-machine transition은 `server.go:156-168`의 `newMachine`에 local이며 shared package를 변경하지 않는다. |
| Go code quality | PASS | 0 | 0 | request context는 `server.go:201`, `server.go:217`, `server.go:239`에서 transition과 guard check에 도달한다. sentinel error는 `errors.Is`와 호환된다. |
| Tests/types/silent failure | PASS | 0 | 0 | test는 valid path, final state, invalid transition, guard rejection, can endpoint, replay, key conflict, failed-key reuse, bad request, negative amount, concurrent duplicate transition을 다룬다. |
| Performance/stability | PASS | 0 | 0 | 유일한 shared mutable state는 `server.go:209-229`에서 transition과 idempotency write 주변을 `Server.mu`로 serialize한다. example package race validation이 통과했다. |
| Documentation/release evidence | PASS | 0 | 0 | English/Korean README는 scenario, Architecture, Sequence Diagram section을 포함하고 root README navigation은 새 example을 포함한다. |

## 근거

| 점검 | 결과 | 근거 |
|---|---|---|
| state model | PASS | requested, authorized, captured, failed, cancelled state는 `server.go:19-30`에 선언되어 있다. allowed transition과 final state는 `server.go:156-168`에 등록되어 있다. |
| positive amount guard | PASS | `positiveAmountGuard`는 `server.go:171-178`에서 non-positive authorization을 거부한다. `TestServerGuardRejectionReturnsConflictAndKeepsState`는 `server_test.go:140-161`에서 rejection과 unchanged state를 다룬다. |
| request context propagation | PASS | handler는 `server.go:201`에서 `c.Request.Context()`를 `transitionWithIdempotency`에 전달한다. transition과 can check는 `server.go:217`, `server.go:239`에서 같은 context를 사용한다. |
| idempotent replay | PASS | replay는 `server.go:213-215`에서 mutation 전에 일어난다. same-key same-event replay는 `server.go:273-283`에서 표시된다. `TestServerIdempotentReplayDoesNotMutateState`는 `server_test.go:184-212`에서 response와 state stability를 다룬다. |
| idempotency conflict | PASS | 같은 key와 다른 event는 `server.go:278-280`에서 wrapped `errIdempotencyKeyReuse`를 반환한다. HTTP는 `server.go:324-325`에서 이를 `409 idempotency_conflict`로 매핑한다. test coverage는 `server_test.go:214-235`에 있다. |
| failed transition replay 방지 | PASS | failed transition은 `server.go:217-220`에서 idempotency entry 저장 전에 반환된다. `TestServerDoesNotStoreFailedTransitionsAsReplay`는 `server_test.go:237-258`에서 이후 successful reuse를 다룬다. |
| final-state와 invalid transition handling | PASS | final state는 `server.go:167`에 등록되어 있다. HTTP는 `server.go:326-333`에서 invalid/final/concurrent transition error를 매핑한다. test는 `server_test.go:78-120`의 final state와 `server_test.go:122-138`의 invalid capture를 다룬다. |
| concurrency safety | PASS | `transitionWithIdempotency`는 `server.go:209-229`에서 replay check, transition, snapshot, store를 `Server.mu`로 serialize한다. `TestServerConcurrentDuplicateTransitionSafety`는 `server_test.go:296-341`에서 concurrent authorize request를 다룬다. |
| HTTP request validation | PASS | JSON binding과 request parsing은 `server.go:188-199`, `server.go:290-317`에서 malformed, missing, blank, unknown event를 거부한다. table coverage는 `server_test.go:260-287`에 있다. |
| README/diagram contract | PASS | example README는 `README.md:11`, `README.md:105`, `README.md:114`에 Example Scenario, Architecture, Sequence Diagram을 포함한다. Korean README도 `README.ko.md:11`, `README.ko.md:104`, `README.ko.md:113`에 이를 반영한다. |

## Diagram Gate 근거

- `bash scripts/generate-payment-authorization-state-diagrams.sh`: PASS
  - `payment-authorization-state-scenario`: `badEndpointAngle=0`, `badBends=0`, `interiorCrossings=0`, `marginImbalance=0`, `titleGap=ok`, `fontFallback=0`
  - `payment-authorization-state-architecture`: `badEndpointAngle=0`, `badBends=0`, `interiorCrossings=0`, `marginImbalance=0`, `titleGap=ok`, `fontFallback=0`
  - `payment-authorization-state-sequence`: `badEndpointAngle=0`, `badBends=0`, `interiorCrossings=0`, `marginImbalance=0`, `titleGap=ok`, `fontFallback=0`
- visual inspection은 다음 asset에서 통과했다.
  - `payment-authorization-state-scenario.png`
  - `payment-authorization-state-architecture.png`
  - `payment-authorization-state-sequence.png`
  - `workshop-example-map.png`
- README는 PNG asset만 embed한다. matching SVG, Graphviz SVG/PNG, DOT, plain artifact는 PNG
  file 옆에 저장되어 있다.

## 리뷰 전 검증 실행

- `codegraph status`: up to date, 44 files, 734 nodes, 1,704 edges.
- `code-review-graph build --repo "$PWD"`: 41 files, 345 nodes, 3,095 edges, 31 flows, 15 communities.
- `go test -count=1 ./examples/payment-authorization-state/...`: PASS
- `go test -race -count=1 ./examples/payment-authorization-state/...`: PASS
- `go test -run '^$' ./examples/payment-authorization-state`: PASS
- production concurrency quick scan: `server_test.go:307`에 `go func` hit 하나가 있다.
  production code가 아니라 의도적인 test stress harness다.
- bad diagram font-family scan for `Inter|Arial|Helvetica`: 새 final SVG asset에서 hit 0건.

## 잔여 위험

- idempotency는 example을 위해 의도적으로 in-memory다. README는 이 pattern을 production에
  적용하기 전에 durable idempotency storage, TTL, request-hash validation, audit logging이
  필요하다고 명시한다.
