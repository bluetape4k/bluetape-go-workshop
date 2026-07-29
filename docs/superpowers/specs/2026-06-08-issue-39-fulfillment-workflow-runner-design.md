# Issue 39 Fulfillment Workflow Runner 설계

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: issue #39는 실행 가능한 새 Gin 예제 디렉터리, Go 코드, 테스트,
  영어/한국어 README 파일, 루트 README 항목, 다이어그램 자산, 리뷰 산출물,
  lesson, PR을 추가한다.
- 저장소: `bluetape4k/bluetape-go-workshop`.
- 브랜치/워크트리:
  `feat/issue-39-fulfillment-workflow-runner`, 위치는
  `.worktrees/feat-issue-39-fulfillment-workflow-runner`.

## 문제

워크숍에는 릴리스된 `workflow` runner가 일반 Go 작업 함수를 애플리케이션
형태의 fulfillment 흐름으로 조합하는 0.4.0 예제가 필요하다. 이 예제는
패키지 API 조각을 넘어야 한다. 독자는 순차 검증, 병렬 부수 효과 분기, 조건부
배송 생성, 취소/실패 보고의 경계를 함께 확인할 수 있어야 한다.

## 현재 근거

- GitHub issue #39는 순차, 병렬, 조건부, 실패, skip, 취소를 다루는
  fulfillment workflow runner 예제를 요구한다.
- `github.com/bluetape4k/bluetape-go/workflow`는 context-aware 작업 함수
  위에서 동작하는 `Sequential`, `Parallel`, `Conditional` runner를 제공한다.
- `github.com/bluetape4k/bluetape-go/workreport`는 workflow runner 결과가
  공유하는 최종 상태와 실패 정책을 제공한다.
- 이 저장소는 이미 0.4.0 공개 HTTP 예제
  `examples/order-lifecycle-state-api`에서 Gin을 사용한다.
- 사용자는 각 예제 README에 `bluetape4k-diagram` 게이트를 따르는 예제
  시나리오, Architecture, Sequence Diagram을 포함하도록 요구했다.

## 목표

- 실행 가능한 `examples/fulfillment-workflow-runner` Gin API를 추가한다.
- 하나의 fulfillment 시나리오 안에서 `workflow.Sequential`,
  `workflow.Parallel`, `workflow.Conditional`을 보여준다.
- 테스트와 README 예제가 실행 경계를 설명할 수 있도록 `workreport.Report`
  트리의 안정적인 JSON projection을 반환한다.
- 성공, 단계 실패, 조건부 배송 skip, 취소를 테스트로 다룬다.
- 예제를 인메모리와 결정적 동작으로 유지한다.
- 0.4.0에 맞춰 루트 README 탐색 항목과 roadmap 문구를 갱신한다.
- 시나리오, Architecture, Sequence Diagram, 실행 명령, endpoint, 실패 의미,
  운영 환경 주의점을 담은 영어/한국어 예제 README 파일을 추가한다.

## 비목표

- 영속 workflow engine, saga coordinator, retry scheduler, message queue,
  database, 외부 payment/inventory service를 구현하지 않는다.
- 이미 존재하는 Gin과 bluetape-go 패키지를 넘어서는 새 런타임 의존성을
  추가하지 않는다.
- 범용 workflow DSL이나 변경 가능한 shared context map을 만들지 않는다.
- 과도한 애플리케이션 scaffolding 뒤에 `workflow` 의미를 숨기지 않는다.

## 제안 예제 형태

디렉터리:

```text
examples/fulfillment-workflow-runner/
  main.go
  README.md
  README.ko.md
  internal/fulfillment/
    server.go
    server_test.go
```

패키지 이름: `fulfillment`.

`Server`는 `http.Handler`를 구현하고 Gin router를 소유하며, 요청마다 새
workflow runner를 만든다. Workflow 상태는 요청 범위에 머물러야 하므로
테스트가 요청 간 race 없이 동작을 증명할 수 있다.

## 도메인 시나리오

Fulfillment 요청은 다음 필드를 포함한다.

- `order_id`
- `stock_available`
- `payment_authorized`
- `requires_shipment`

API는 다음 흐름을 모델링한다.

1. 순차 `fulfillment` runner가 `validate-order`로 시작한다.
2. 병렬 `risk-checks` runner가 다음 작업을 실행한다.
   - `reserve-inventory`
   - `authorize-payment`
3. 조건부 `shipment-decision` runner는 `requires_shipment`가 true일 때만
   배송을 생성한다. 그 외에는 배송을 만들지 않고 skip된 배송 분기를 완료로
   기록한다.

## HTTP API

모든 route에는 Gin을 사용한다.

| Method | Path | 동작 |
| --- | --- | --- |
| `GET` | `/healthz` | 서비스 health를 반환한다. |
| `POST` | `/fulfillment/run` | 시나리오 입력을 bind하고 workflow를 실행한 뒤 report tree를 반환한다. |

응답 규칙:

- `200 OK`: workflow가 성공적으로 완료되었거나 의도적인 조건부 skip을 포함해
  완료되었다.
- `409 Conflict`: 일반 분기 실패 때문에 workflow가 실패했거나 부분 완료
  상태가 되었다.
- `408 Request Timeout`: 호출자 context 취소나 deadline 때문에 취소된
  workflow report가 생성되었다.
- `400 Bad Request`: 잘못된 JSON 또는 유효하지 않은 요청 필드다.

## Report Projection

테스트와 README 예제가 안정적으로 사용할 수 있는 JSON 형태를 노출한다.

- workflow 이름
- 상태
- 선택적 error message
- 선택적 reason
- 재귀적인 child report
- 파생 flag:
  - `success`
  - `failure`
  - `cancelled`

API는 JSON 응답에 timestamp를 노출하면 안 된다. `workreport.Report`의
timestamp는 의도적으로 런타임 값이며, README snippet과 테스트를 불필요하게
불안정하게 만든다.

## 오류 및 취소 계약

- Validation failure는 일반 failed report이며, 요청 형태는 유효하지만
  도메인이 진행할 수 없을 때 `409`로 mapping한다.
- Inventory reservation failure와 payment authorization failure는 병렬 분기
  아래 named child report로 보여야 한다.
- `StopOnFailure`를 사용하는 `workflow.Parallel`은 실패 시 sibling work를
  취소해야 한다. 테스트는 느린 sibling이 공유 context 취소를 관찰할 때
  cancellation child가 생기는지 검증해야 한다.
- 취소된 caller context는 `cancelled` report를 만들고 `408`로 mapping해야
  한다.
- 잘못된 JSON과 유효하지 않은 필드는 workflow에 들어가면 안 되며 `400`을
  반환한다.

## 설계 선택지

### Option A - 요청 범위 Workflow를 사용하는 단일 Gin Endpoint

요청마다 runner를 만들고 report tree를 반환한다.

장점:

- 작고 시나리오 중심이다.
- 요청 간 공유 상태가 없다.
- 순차, 병렬, 조건부 의미를 직접 드러낸다.

비용:

- 운영 환경용 orchestration engine은 아니다.

### Option B - Domain Function만 제공

`RunFulfillment(ctx, input)`과 테스트만 노출한다.

장점:

- 코드가 더 작다.

비용:

- 워크숍의 공개 HTTP API 방향과 맞지 않고 실행 가능한 서비스를 제공하지
  못한다.

### Option C - Durable Fulfillment Store

메모리 또는 영속 저장소에서 workflow 실행과 상태를 추적한다.

장점:

- 운영 환경의 workflow UX에 더 가깝다.

비용:

- `workflow` runner와 무관한 저장소와 lifecycle ownership을 추가한다.

## 결정

Option A를 채택한다.

이 issue의 핵심은 lightweight runner 의미를 보여주는 것이다. 요청 범위 Gin
workflow는 예제를 실행 가능하고 테스트 가능하며 명확하게 유지하면서,
영속성과 orchestration 관심사를 이후 통합 예제로 미룬다.

기각:

- Option B: issue #39와 워크숍 방향이 공개 Gin 예제를 요구하기 때문이다.
- Option C: persistence가 runner 계약에서 주의를 분산시키고 `workflow`가
  소유하지 않는 failure mode를 추가하기 때문이다.

## 테스트 전략

집중 package test:

- health endpoint가 OK를 반환한다.
- 성공 요청은 validate, 병렬 risk check, 배송 생성 child를 포함한 completed
  workflow를 반환한다.
- 조건부 skip 요청은 배송 생성 없이 completed workflow를 반환한다.
- inventory failure는 `409`, parent failed/partial status, 가시적인 child
  failure를 반환한다.
- payment failure는 `StopOnFailure` 아래의 느린 inventory sibling을 취소한다.
- caller cancellation은 `408`과 cancelled report를 반환한다.
- 잘못된 JSON과 유효하지 않은 요청 필드는 `400`을 반환한다.

검증 명령:

- `bash scripts/generate-fulfillment-workflow-diagrams.sh`
- `go test -count=1 ./examples/fulfillment-workflow-runner/...`
- `go test -race -count=1 ./examples/fulfillment-workflow-runner/...`
- `go test -run '^$' ./examples/fulfillment-workflow-runner`
- `go test -count=1 ./...`
- `git diff --check`
- `make ci`

## 문서 및 다이어그램 영향

- `examples/fulfillment-workflow-runner/README.md`를 추가한다.
- `examples/fulfillment-workflow-runner/README.ko.md`를 추가한다.
- 생성된 README 다이어그램 자산을 추가한다.
  - 시나리오
  - architecture
  - sequence
- `bluetape4k-diagram`을 따르는 결정적 gate summary와 함께
  `scripts/generate-fulfillment-workflow-diagrams.sh`를 추가한다.
- 루트 `README.md`와 `README.ko.md`의 예제 표와 0.4.0 실행 지침을 갱신한다.
- README는 다음 내용을 설명해야 한다.
  - workflow step 경계
  - failure policy 의미
  - cancellation propagation
  - conditional skip 동작
  - 운영 환경 hardening 공백

## 위험

1. **내구성 과장**: `workflow`는 durable workflow engine이 아니다. README와
   API 문구는 durable orchestration claim을 피해야 한다.
2. **약한 취소 테스트**: 작업 시작 전에 취소하는 테스트는 sibling cancellation을
   증명하지 못한다. 한 분기가 실패하고 다른 분기가 context cancellation을
   관찰하는 병렬 분기 테스트를 포함한다.
3. **시끄러운 report timestamp**: 원본 `workreport.Report`를 반환하면 동적
   timestamp가 노출된다. 안정적인 response projection을 사용한다.
4. **다이어그램 리뷰 누락**: 시나리오와 sequence 다이어그램은 connector가
   많다. PNG/SVG 쌍을 생성하고 결정적 geometry gate summary를 실행하며,
   PR 전에 렌더링된 PNG를 검사한다.
