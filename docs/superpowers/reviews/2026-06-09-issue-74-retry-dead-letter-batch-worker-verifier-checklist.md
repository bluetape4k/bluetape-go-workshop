# Issue #74 verifier checklist

범위:

- milestone 0.5.0용 retry/dead-letter batch worker 예제.
- documentation, README diagram asset, root navigation, PR-ready evidence.

## Gate 상태

| Gate | 상태 | 근거 |
|---|---|---|
| Research | PASS | `docs/superpowers/research/2026-06-09-issue-74-retry-dead-letter-batch-worker-research.md` |
| Spec | PASS | `docs/superpowers/specs/2026-06-09-issue-74-retry-dead-letter-batch-worker-design.md` |
| Step 2-R | PASS | `docs/superpowers/reviews/2026-06-09-issue-74-retry-dead-letter-batch-worker-spec-review.md`, P0=0/P1=0 |
| Plan | PASS | `docs/superpowers/plans/2026-06-09-issue-74-retry-dead-letter-batch-worker-plan.md` |
| Step 3-R | PASS | `docs/superpowers/reviews/2026-06-09-issue-74-retry-dead-letter-batch-worker-plan-review.md`, P0=0/P1=0 |
| Implementation | PASS | `examples/retry-dead-letter-batch-worker/internal/ticketworker/worker.go`와 test가 추가됐다. |
| README content | PASS | EN/KO README는 scenario, Architecture, Sequence Diagram, run command, test, related example, hardening note를 포함한다. |
| Diagram assets | PASS | scenario, architecture, sequence diagram에 대해 DOT, PLAIN, Graphviz SVG/PNG, decorated SVG/PNG asset이 생성됐다. |
| Step 6-R | PASS | `docs/superpowers/reviews/2026-06-09-issue-74-retry-dead-letter-batch-worker-code-review.md`, P0=0/P1=0 |

## 명령 근거

final commit 전 refresh:

| 명령 | 결과 |
|---|---|
| `bash scripts/generate-retry-dead-letter-batch-worker-diagrams.sh` | PASS. scenario, architecture, sequence gate가 모두 `badEndpointAngle=0`, `badBends=0`, `interiorCrossings=0`, `marginImbalance=0`, `margins=44/44/34/34`, `titleGap=ok`, `fontFallback=0`을 보고했다. |
| PNG visual inspection | PASS. scenario, architecture, sequence PNG에는 decorator frame, balanced margin, readable label, path를 구분하는 line color가 있다. |
| `go test -count=1 ./examples/retry-dead-letter-batch-worker/...` | PASS |
| `go test -count=1 -run 'Stress\|Concurrent' ./examples/retry-dead-letter-batch-worker/internal/ticketworker` | PASS. concurrent run isolation과 concurrent store access stress coverage. |
| `go test -race -count=1 ./examples/retry-dead-letter-batch-worker/...` | PASS |
| `go test -race -count=1 -run 'Stress\|Concurrent' ./examples/retry-dead-letter-batch-worker/internal/ticketworker` | PASS. stress coverage도 race detector와 함께 통과했다. |
| `go run ./examples/retry-dead-letter-batch-worker` | PASS. deterministic JSON이 expected count와 DLT entry를 출력했다. |
| `go test -run '^$' ./examples/retry-dead-letter-batch-worker` | PASS |
| `go vet ./examples/retry-dead-letter-batch-worker/...` | PASS |
| `golangci-lint run ./examples/retry-dead-letter-batch-worker/...` | PASS; `0 issues.` |
| `git diff --check` | PASS |
| `golangci-lint cache clean && make ci` | PASS. repository lint와 모든 example test가 성공적으로 완료됐다. |

demo output summary:

- `status=completed`
- `read_count=4`
- `write_count=3`
- `retry_count=1`
- `skip_count=1`
- `dead_letters[0].ticket_id=ticket-1003`

## 잔여 위험

example은 의도적으로 in-memory queue, sink, dead-letter storage를 사용한다. production durability,
idempotent replay, backoff jitter, metric, alerting은 hardening topic으로 문서화되어 있으며
이 issue의 범위 밖이다.
