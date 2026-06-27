# dynamodb-batchwrite-materializer

[English](README.md) | [한국어](README.ko.md)

`dynamodb/batchwrite`를 위한 document index materializer 예제입니다.

이 예제는 흔한 application boundary를 다룹니다. Document service가 많은 domain
event를 만들고, projection worker가 그 event들을 DynamoDB index table에 씁니다.
DynamoDB `BatchWriteItem`은 한 번에 최대 25개 write request만 받고, capacity가
일시적으로 부족하면 `UnprocessedItems`를 반환할 수 있습니다. Worker business code가
이 규칙을 직접 다시 구현하면 retry와 error policy가 쉽게 흩어집니다.

이 예제는 domain mapping은 application에 두고, wire-level batch 동작은 bluetape-go에
맡깁니다.

- application은 `DocumentEvent`를 AWS SDK v2 `types.WriteRequest`로 변환합니다.
- `batchwrite.WriteAll`은 25-item chunking과 `UnprocessedItems` retry를 담당합니다.
- retry budget을 모두 사용해도 남은 item이 있으면 application error로 보고하되,
  bluetape-go `batchwrite.UnprocessedItemsError`도 그대로 보존합니다.
- typed AWS service error와 context cancellation은 caller가 구분할 수 있게 유지합니다.

## 시나리오

Sample input은 한 tenant의 document event 30개입니다. Materializer는 event 하나를
DynamoDB `PutRequest` 하나로 바꿉니다. 따라서 실제 DynamoDB 호출은 두 번으로
나뉩니다. 첫 chunk는 25개, 둘째 chunk는 5개입니다.

Materializer가 쓰는 index row shape는 다음과 같습니다.

| Attribute | 예 | 이유 |
|---|---|---|
| `pk` | `TENANT#tenant-alpha` | Tenant별 document를 묶습니다. |
| `sk` | `DOC#doc-001#v000001` | Document version을 정렬 가능하게 둡니다. |
| `document_id` | `doc-001` | Domain identifier를 보존합니다. |
| `tenant_id` | `tenant-alpha` | Tenant metadata를 검사 가능하게 둡니다. |
| `version` | `1` | Projection version을 DynamoDB number로 저장합니다. |
| `title` | `Document 001` | 일반 string attribute를 보여줍니다. |
| `body_hash` | `sha256:...0001` | 변경 불가능한 content metadata를 보여줍니다. |

## Architecture

![DynamoDB batch write materializer architecture](../../docs/images/readme-diagrams/dynamodb-batchwrite-materializer-architecture.png)

Architecture는 세 책임을 나눕니다.

1. Domain worker는 event validation과 document event에서 DynamoDB write request로
   바꾸는 mapping을 소유합니다.
2. `dynamodb/batchwrite`는 AWS batch mechanics를 소유합니다. 최대 25개 제한,
   retry budget, backoff, `UnprocessedItems` loop가 여기에 있습니다.
3. Production infrastructure는 table capacity, alarm, dead-letter policy, replay
   scheduling을 소유합니다.

이 분리가 핵심 lesson입니다. Application은 DynamoDB batch-write retry loop를 다시
구현하지 않고도 item shape와 error policy를 code review할 수 있습니다.

## Retry Sequence

![DynamoDB batch write retry sequence](../../docs/images/readme-diagrams/dynamodb-batchwrite-materializer-retry-sequence.png)

`BatchWriteItem` failure는 크게 다른 범주로 나뉩니다.

| 결과 | 예제 동작 | Caller가 보는 것 |
|---|---|---|
| Partial success | DynamoDB가 일부 item을 `UnprocessedItems`로 반환합니다. | `batchwrite.WriteAll`이 그 item만 다시 보냅니다. |
| Retry exhaustion | 설정된 attempt 이후에도 `UnprocessedItems`가 남아 있습니다. | `ErrRetryExhausted`를 반환하면서도 `batchwrite.ErrUnprocessedItems`와 `UnprocessedItemsError`를 `errors.Is` / `errors.As`로 찾을 수 있습니다. |
| Service error | AWS가 `ProvisionedThroughputExceededException` 같은 typed error를 반환합니다. | AWS SDK typed error를 보존하고 retry exhaustion으로 바꾸지 않습니다. |
| Cancellation | Retry가 계속되기 전에 caller가 context를 취소합니다. | `context.Canceled` 또는 `context.DeadlineExceeded`가 보존됩니다. |

이 구분은 production에서 중요합니다. Retry exhaustion은 replay나 dead-letter로 넘길
결정입니다. Typed AWS service error는 보통 infrastructure나 capacity 신호입니다.
Context cancellation은 caller가 ownership을 회수했다는 신호입니다.

## 보여주는 것

- `batchwrite.WriteAll`이 30개 request를 DynamoDB-safe batch로 chunking하는 방식.
- 전체 원본 request가 아니라 반환된 `UnprocessedItems`만 retry하는 방식.
- Retry exhaustion과 AWS SDK service error를 분리하는 error policy.
- Batch helper를 지나도 context cancellation을 보존하는 구조.
- Retry와 error behavior를 deterministic fake client로 검증하는 방법.
- 실제 AWS SDK v2 client path를 확인하는 opt-in Floci DynamoDB smoke test.

## 실행

Local preview를 출력합니다.

```bash
go run ./examples/dynamodb-batchwrite-materializer
```

Output은 JSON이며 DynamoDB에 접속하지 않습니다.

```json
{
  "table": "document-index",
  "event_count": 30,
  "request_count": 30,
  "chunk_count": 2,
  "chunk_limit": 25,
  "retry_budget": 3
}
```

실제 output에는 boundary note, smoke-test command, example document ID도 들어
있습니다. Preview는 일부러 local로 유지했습니다. Reader가 Docker나
AWS-compatible service를 시작하기 전에 batch-write boundary를 먼저 이해할 수 있게
하기 위해서입니다.

## 테스트

Deterministic test를 실행합니다.

```bash
go test -count=1 ./examples/dynamodb-batchwrite-materializer/...
go test -race -count=1 ./examples/dynamodb-batchwrite-materializer/...
```

Targeted test는 다음을 증명합니다.

- 30개 event가 두 DynamoDB call, `25 + 5`로 나뉩니다.
- Retry call에는 반환된 `UnprocessedItems`만 들어갑니다.
- Retry exhaustion은 `ErrRetryExhausted`를 반환하고
  `batchwrite.UnprocessedItemsError`를 보존합니다.
- Context cancellation이 전파됩니다.
- Typed AWS service error를 `errors.As`로 찾을 수 있습니다.
- Invalid event는 DynamoDB client를 호출하기 전에 실패합니다.

## Optional Floci Smoke Test

Floci-backed smoke test는 opt-in입니다. Docker-backed AWS service emulation을
시작하므로 다른 container-backed suite와 serial로 실행하는 것이 좋습니다.

```bash
BLUETAPE_DYNAMODB_BATCHWRITE_SMOKE=1 go test -run TestFlociSmoke -count=1 ./examples/dynamodb-batchwrite-materializer/...
```

Smoke test는 DynamoDB table을 만들고, 같은 materializer를 실제 AWS SDK v2 client로
실행한 뒤 table scan으로 30개 index row가 쓰였는지 확인합니다.

## Boundary Notes

- Item-shape mapping은 application code에 둡니다. Helper가 domain key나 attribute
  name을 결정하면 안 됩니다.
- Retry budget과 backoff는 worker boundary에서 조정합니다. Table capacity와 replay
  policy는 generic helper가 아니라 production operations 책임입니다.
- 모든 DynamoDB error를 retry exhaustion으로 취급하지 않습니다. Capacity, permission,
  validation failure를 진단할 수 있게 AWS SDK typed error를 보존합니다.
- Chunking, retry, cancellation, wrapping behavior는 deterministic fake로 검증하고,
  container-backed smoke test는 opt-in으로 유지합니다.
