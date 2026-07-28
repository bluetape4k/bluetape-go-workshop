# Step 6-R 코드 리뷰: Issue #74 Retry Dead-Letter Batch Worker

검토 범위:

- `examples/retry-dead-letter-batch-worker/**`
- Root `README.md` and `README.ko.md` navigation updates
- `docs/images/readme-diagrams/retry-dead-letter-batch-worker-*`
- `docs/images/readme-diagrams/workshop-example-map.*`
- `scripts/generate-retry-dead-letter-batch-worker-diagrams.sh`

reviewer stance:

- `bluetape4k-full-feature` Step 6-R implementation review.
- `bluetape-go-patterns` Go P0/P1 gate.
- `bluetape4k-diagram` README diagram gate.

## finding

P0/P1 blocker는 발견되지 않았다.

| Tier | 영역 | P0 | P1 | P2 | P3 | 근거 |
|---|---|---:|---:|---:|---:|---|
| 1 | Security | 0 | 0 | 0 | 0 | example에는 auth, secret, shell, external network, unsafe deserialization, path boundary가 없다. |
| 2 | Ops/SRE reliability | 0 | 0 | 0 | 0 | retry와 skip budget이 명시적이다. cancellation은 retry되거나 dead-lettered되지 않는다. production durability caveat이 문서화되어 있다. |
| 3 | Structural impact | 0 | 0 | 0 | 0 | 새 isolated example과 README/map asset만 포함한다. shared bluetape-go API나 module dependency는 변경되지 않았다. |
| 4 | Go code quality | 0 | 0 | 0 | 0 | `batch.RetryPolicy`, `batch.SkipPolicy`, `context.Context`, `errors.Is`와 함께 쓰는 sentinel error, mutex-protected in-memory store를 사용한다. |
| 5 | Tests/types/silent failure | 0 | 0 | 0 | 0 | test는 transient retry success, permanent DLT, skip exhaustion, writer failure, cancellation, bounded stress, race, stable report projection을 다룬다. |
| 6 | Performance/stability | 0 | 0 | 0 | 0 | fixture, retry attempt, concurrent run/store stress가 bounded하다. goroutine/timer/external IO는 도입되지 않았고 race test가 통과한다. |
| 7 | Docs/evidence | 0 | 0 | 0 | 0 | EN/KO README는 scenario, Architecture, Sequence Diagram, policy table, related example, production hardening을 포함한다. |

## Diagram Gate

`bash scripts/generate-retry-dead-letter-batch-worker-diagrams.sh`:

- `scenario: nodes=8 routes=6 segments=6 badEndpointAngle=0 badBends=0 interiorCrossings=0 marginImbalance=0 margins=44/44/34/34 titleGap=ok fontFallback=0`
- `architecture: nodes=9 routes=9 segments=15 badEndpointAngle=0 badBends=0 interiorCrossings=0 marginImbalance=0 margins=44/44/34/34 titleGap=ok fontFallback=0`
- `sequence: nodes=5 routes=9 segments=12 badEndpointAngle=0 badBends=0 interiorCrossings=0 marginImbalance=0 margins=44/44/34/34 titleGap=ok fontFallback=0`

PNG visual inspection:

- `retry-dead-letter-batch-worker-scenario.png`: decorator frame이 있고 margin이 balanced하며 label이 route와 overlap되지 않는다.
- `retry-dead-letter-batch-worker-architecture.png`: decorator frame이 있고 layer band를 읽을 수 있으며 semantic line color가 구분된다.
- `retry-dead-letter-batch-worker-sequence.png`: decorator frame이 있고 top/bottom/left/right margin이 balanced하며 sequence label이 읽을 수 있게 남아 있다.
- `workshop-example-map.png`: 새 example node가 batch lane 아래에 나타나며 existing map style에서 계속 읽을 수 있다.

## Static Scan

`rg -n "context\\.TODO\\(|time\\.Tick\\(|go func|panic\\(|http\\.ListenAndServe\\(|RealIP|X-Forwarded-For" examples/retry-dead-letter-batch-worker || true`

결과: match 없음.

CodeGraph는 이 worktree에서 initialized되지 않았다. 이 gap은 새 package direct review, `batch`의 local
module source, repository-local example pattern으로 완화했다.

## 검증 근거

이 review 전에 이미 실행한 항목:

- `go test -count=1 ./examples/retry-dead-letter-batch-worker/...` PASS
- `go test -count=1 -run 'Stress|Concurrent' ./examples/retry-dead-letter-batch-worker/internal/ticketworker` PASS
- `go test -race -count=1 ./examples/retry-dead-letter-batch-worker/...` PASS
- `go test -race -count=1 -run 'Stress|Concurrent' ./examples/retry-dead-letter-batch-worker/internal/ticketworker` PASS
- `go run ./examples/retry-dead-letter-batch-worker` PASS. output은 `read=4 write=3 retry=1 skip=1`과 `ticket-1003`에 대한 DLT entry 하나를 보고한다.
- `go test -run '^$' ./examples/retry-dead-letter-batch-worker` PASS
- `go vet ./examples/retry-dead-letter-batch-worker/...` PASS
- `golangci-lint run ./examples/retry-dead-letter-batch-worker/...` PASS, `0 issues`

final validation은 이 review file을 추가한 뒤 verifier checklist에 기록된다.

## 판정

P0=0 P1=0. Step 6-R이 통과했다.
