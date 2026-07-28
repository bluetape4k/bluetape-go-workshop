# Issue #74 설계: Retry and Dead-Letter Batch Worker 예제

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: 새 실행 가능한 예제 디렉터리, Go 구현, 테스트, 영어 및 한국어 README
  파일, 생성된 README 다이어그램, 루트 navigation 갱신, review 산출물, PR.
- 저장소: `bluetape4k/bluetape-go-workshop`.
- 브랜치/워크트리:
  `feat/issue-74-retry-dead-letter-batch-worker`, 위치는
  `.worktrees/feat-issue-74-retry-dead-letter-batch-worker`.

## 목표

Durable queue를 도입하지 않으면서 bounded retry, transient와 permanent work-item
failure 분류, domain-visible dead-letter 처리를 보여주는 집중 v0.5.0 batch
예제를 추가한다.

## 비목표

- Gin, background scheduling, leader election, NATS, Redis, database를 추가하지
  않는다.
- generic queue framework를 구현하지 않는다.
- `batch.Step` 밖에 새 retry library나 custom retry loop를 추가하지 않는다.
- in-memory queue나 dead-letter list가 production durable하다고 주장하지 않는다.
- checkpointing으로 failed writer chunk를 skip하지 않는다.

## 예제

- 경로: `examples/retry-dead-letter-batch-worker`
- 패키지: `internal/ticketworker`
- 실행 entrypoint: `main.go`
- Package dependency focus: `batch`, timestamp-free report projection 포함.

## 시나리오

Support operations team이 queued ticket notification을 대상으로 batch worker를
실행한다. 대부분의 ticket은 한 번에 처리된다. 하나의 ticket은 첫 시도에서
transient하게 실패하고 retry에서 성공한다. 또 다른 ticket은 delivery address가
blocked 상태라 영구적으로 유효하지 않다. Processor는 ticket ID, classification,
attempts, reason을 담은 dead-letter entry를 기록한 뒤 skippable permanent
error를 반환한다.

완료된 batch report는 다음을 보여준다.

- 모든 input ticket을 읽었다.
- 유효한 ticket을 썼다.
- permanent ticket 하나를 skip했다.
- retry count는 transient retry만 반영한다.
- dead-letter record는 skip된 item과 reason을 보존한다.

## 도메인 모델

```go
type Ticket struct {
    ID       string
    Channel  string
    Address  string
    Scenario FailureScenario
}

type ProcessedTicket struct {
    ID      string `json:"id"`
    Channel string `json:"channel"`
    Address string `json:"address"`
    Attempts int   `json:"attempts"`
}

type DeadLetter struct {
    TicketID string `json:"ticket_id"`
    Reason   string `json:"reason"`
    Attempts int    `json:"attempts"`
}
```

`FailureScenario`는 예제 내부에만 있으며 success, transient-once, permanent라는
결정적 fixture 동작을 지원한다.

## Batch 설계

Reader:

- 결정적인 in-memory slice를 읽는다.
- open 중 비어 있지 않은 ticket ID를 검증한다.
- 다음 index를 추적한다.
- `Open`, `Read`, `Close`에서 `ctx.Err()`를 확인한다.
- 테스트를 위해 close state를 기록한다.

Processor:

- ticket field를 검증한다.
- ticket별 attempt를 메모리에서 추적한다.
- attempt가 설정된 transient-success threshold보다 낮으면 transient sentinel
  error를 반환한다.
- permanent sentinel error를 반환하기 전에 `DeadLetter`를 기록한다.
- `context.Canceled` 또는 `context.DeadlineExceeded`는 retry하지 않는다.
- `errors.Is` check를 위해 sentinel error를 ticket context와 함께 wrap한다.

Writer:

- processed ticket을 in-memory sink에 저장한다.
- 처리 순서를 보존한다.
- duplicate ticket ID를 wrapped error로 거부한다.
- 쓰기 전과 쓰기 중 context를 확인한다.
- 테스트를 위해 close state를 기록한다.

Runner:

- 다음 값으로 `batch.Step[Ticket, ProcessedTicket]`를 만든다.
  - `Name: "ticket-retry-dead-letter-worker"`
  - `ChunkSize: 2`
  - `RetryPolicy: batch.RetryErrors(3, errors.Is(err, ErrTransientTicket))`
  - `SkipPolicy: batch.SkipErrors(2, errors.Is(err, ErrPermanentTicket))`
- step을 `ticket-batch-worker` 이름의 `batch.Job`으로 wrap한다.
- timestamp 없는 결정적 JSON projection을 반환한다.

## CLI 출력

`go run ./examples/retry-dead-letter-batch-worker`는 다음과 비슷한 안정적인 JSON을
출력한다.

```json
{
  "job_name": "ticket-batch-worker",
  "chunk_size": 2,
  "status": "completed",
  "read_count": 4,
  "write_count": 3,
  "retry_count": 1,
  "skip_count": 1,
  "processed": ["ticket-1001", "ticket-1002", "ticket-1004"],
  "dead_letters": [
    {"ticket_id": "ticket-1003", "reason": "blocked address", "attempts": 1}
  ]
}
```

정확한 field name은 구현 중 바뀔 수 있지만, 출력은 timestamp-free이고 결정적이어야
한다.

## 실패 및 취소 계약

- Transient ticket failure는 한 번 retry한 뒤 성공하고
  `batch.Report.RetryCount`를 증가시킨다.
- Permanent ticket failure는 dead-letter entry를 기록하고 `batch.SkipPolicy`로
  skip되며, `SkipCount`를 증가시키고 job을 중단하지 않는다.
- Skip budget을 초과하면 batch가 실패하고 첫 permanent error를 보존한다.
- 이 예제에서 writer error는 dead-letter event로 사용하지 않으므로 duplicate
  writer output은 batch를 실패시킨다.
- Caller cancellation은 `batch.StatusCancelled`를 반환하고 열린 resource를
  닫으며, cancellation을 retry하지 않고 cancelled item의 dead-letter entry를
  만들지 않는다.

## 다이어그램

`docs/images/readme-diagrams/` 아래에 README 다이어그램 자산을 생성한다.

- `retry-dead-letter-batch-worker-scenario`
- `retry-dead-letter-batch-worker-architecture`
- `retry-dead-letter-batch-worker-sequence`

README 파일은 PNG만 embed한다. SVG 파일은 review를 위해 PNG 파일 옆에 유지한다.
Graphviz `.dot`, `.plain`, `*-graphviz.svg`, `*-graphviz.png`는 route evidence로
남긴다. 최종 README SVG/PNG 자산은 outer frame, title/subtitle, content band
또는 panel, footer callout, pastel card, semantic connector color, 구체적인
`margins=L/R/T/B` gate output을 포함하는 decorated workshop baseline을 사용해야
한다.

## 문서

영어 및 한국어 README 파일에 다음을 추가한다.

- Example Scenario
- local batch demonstration 실행 방법
- retry 및 dead-letter policy table
- output shape
- Architecture
- Sequence Diagram
- #40 operations failure policy 및 #73 checkpoint/restart와의 관계
- durable queue, persistent DLT storage, idempotent side effect, backoff/jitter,
  metric, alerting, replay tool에 대한 production hardening note

루트 `README.md`와 `README.ko.md`를 갱신한다.

- example table row
- run section
- 필요 시 0.5.0 roadmap wording
- workshop example map diagram

## 테스트

집중 테스트는 다음을 다뤄야 한다.

- successful demo가 processed ticket 세 개, dead letter 하나, retry count `1`,
  skip count `1`로 완료되는지
- transient item이 retry 이후 성공하고 attempt evidence를 유지하는지
- permanent item이 dead letter에 정확히 한 번 기록되고 skip되는지
- skip budget exhaustion이 batch를 실패시키고 `ErrPermanentTicket`을 wrap하는지
- writer duplicate error가 실패하며 dead-letter record로 변환되지 않는지
- 작업 전 cancellation이 `batch.StatusCancelled`를 반환하고 resource를 닫으며,
  어떤 write나 dead-letter도 만들지 않는지
- transient retry 중 cancellation이 `batch.StatusCancelled`를 반환하고 caller-owned
  cancellation을 retry하지 않는지
- report projection이 런타임 timestamp를 생략하는지
- `go test -race -count=1 ./examples/retry-dead-letter-batch-worker/...`.

## 검증

- `bash scripts/generate-retry-dead-letter-batch-worker-diagrams.sh`
- visual inspection of changed PNG assets
- `go test -count=1 ./examples/retry-dead-letter-batch-worker/...`
- `go test -race -count=1 ./examples/retry-dead-letter-batch-worker/...`
- `go run ./examples/retry-dead-letter-batch-worker`
- `go test -run '^$' ./examples/retry-dead-letter-batch-worker`
- `git diff --check`
- `golangci-lint cache clean && make ci`

## Step 2 Checklist 완료 보고

| 항목 | 상태 | 메모 |
|---|---|---|
| Target repository confirmed | 완료 | `bluetape4k/bluetape-go-workshop`, worktree path 기록. |
| User intent and boundary clear | 완료 | #74 focused retry/dead-letter batch example 구현. |
| Current source evidence checked | 완료 | Issue #74/#29, `bluetape-go v0.5.1` batch/workreport API, #73/#40 예제. |
| Non-goals recorded | 완료 | Durable queue, DB, Gin, scheduler, custom retry framework 없음. |
| Error and cancellation contracts explicit | 완료 | Transient, permanent, skip exhaustion, writer error, cancellation. |
| README diagram requirements explicit | 완료 | PNG embed, SVG sibling, Graphviz evidence, decorated baseline, margin gate. |
| Test expectations explicit | 완료 | Success, failure, cancellation, projection, race validation. |
