# Issue #40 설계: Operations Report Policy 예제

## 목표

`workreport.Report`에서 결정적인 애플리케이션-facing 출력을 만드는 방법과
`workreport.FailurePolicy`가 aggregate 동작을 어떻게 바꾸는지 설명하는
v0.4.0 워크숍 예제를 추가한다.

## 비목표

- durable workflow engine, retry scheduler, queue, database, observability
  의존성을 도입하지 않는다.
- issue #39에서 다룬 workflow runner composition을 다시 설명하지 않는다.
- `workreport.Report`를 변경하거나 런타임 timestamp를 API contract로 노출하지
  않는다.

## 예제

- 경로: `examples/operations-report-policy`
- 패키지: `internal/operations`
- HTTP framework: Gin
- 기본 포트: `:8085`

## API

### `GET /healthz`

`{"status":"ok"}`와 함께 `200 OK`를 반환한다.

### `POST /operations/report`

요청:

```json
{
  "run_id": "release-2026-06-08",
  "policy": "continue_on_failure",
  "products_valid": false,
  "retry_partner_notification": true,
  "skip_search_index": true
}
```

규칙:

- `run_id`는 공백 trim 이후 필수다.
- `policy`는 `stop_on_failure`와 `continue_on_failure`를 허용한다.
- `products_valid=false`는 `validate-products`를 실패시킨다.
- `retry_partner_notification=true`는 `notify-partner`가 실패한 첫 시도와
  완료된 retry를 nested partial report로 보존하게 한다.
- `skip_search_index=true`는 caller-skip reason과 함께 `refresh-search-index`를
  `aborted`로 기록한다.
- 미리 취소된 request context는 cancelled root report를 반환한다.

응답:

```json
{
  "run_id": "release-2026-06-08",
  "policy": "continue_on_failure",
  "status": "partial",
  "summary": {
    "completed": 3,
    "failed": 2,
    "partial": 2,
    "aborted": 1,
    "cancelled": 0,
    "total": 8,
    "terminal": true,
    "success": false,
    "failure": true
  },
  "report": {
    "name": "operations-run",
    "status": "partial",
    "children": []
  }
}
```

HTTP status mapping:

- `completed` -> `200 OK`
- `partial` -> `207 Multi-Status`
- `failed` or `aborted` -> `409 Conflict`
- `cancelled` -> `408 Request Timeout`
- invalid JSON/request/policy -> `400 Bad Request`

## Report Projection

응답은 안정적인 DTO를 사용한다.

- `name`
- `status`
- `error`
- `reason`
- `success`
- `failure`
- `partial`
- `cancelled`
- `children`

`StartedAt`과 `EndedAt`은 런타임 사실이며 예제 assertion을 불안정하게 만들기
때문에 DTO에서 제외한다.

## 다이어그램

`docs/images/readme-diagrams/` 아래에 PNG와 SVG 자산을 생성하고 commit한다.

- `operations-report-policy-scenario`
- `operations-report-policy-architecture`
- `operations-report-policy-sequence`

README는 PNG만 embed하고, 생성된 다이어그램의 label은 영어로 유지한다.

## 테스트

집중 테스트는 다음을 다뤄야 한다.

- health endpoint
- completed run
- continue-on-failure가 validation failure, retry evidence, skip report,
  결정적 summary count를 보존하는지
- stop-on-failure가 첫 failed child 이후 children을 자르는지
- retry report 구조
- 유효하지 않은 policy와 request body
- 미리 취소된 request context가 cancelled report로 mapping되는지
- 예제 package 대상 race test

## 운영 환경 Hardening 메모

README hardening gap은 다음을 명시해야 한다.

- 호출자가 audit history를 필요로 하면 report를 영속화한다.
- demo `run_id`를 넘어서는 operation ID/correlation ID를 부여한다.
- service boundary에서 report DTO를 log/metric/trace와 연결한다.
- retry가 요청 경계를 넘는다면 실제 retry policy/scheduler를 사용한다.
- 도메인에 별도 상태 범주가 필요하면 caller-skipped work와 operator-aborted
  work를 구분한다.
