# s3-sqs-dynamodb-document-workflow

[English](README.md) | [한국어](README.ko.md)

AWS SDK v2, S3, SQS, DynamoDB, `testcontainers/floci`를 함께 사용하는 local
document ingestion workflow 예제입니다.

이 예제는 다음 AWS/Floci 기초 예제를 합치는 통합 단계입니다.

- [`s3-floci-storage`](../s3-floci-storage/README.ko.md)는 안전한 S3 object
  key, metadata, download, response body close를 설명합니다.
- [`sqs-floci-worker`](../sqs-floci-worker/README.ko.md)는 SQS at-least-once
  delivery, delete acknowledgement, retry visibility를 설명합니다.
- [`dynamodb-conditional-repository`](../dynamodb-conditional-repository/README.ko.md)는
  DynamoDB conditional write와 typed conflict 처리를 설명합니다.

여기서는 넓은 AWS wrapper를 만들지 않습니다. Application이 document key
규칙, event shape, idempotency rule을 소유하고, AWS SDK client와
infrastructure는 caller가 소유합니다.

## Scenario

Tenant가 document를 업로드하면 workflow는 다음 순서로 동작해야 합니다.

1. Document body를 S3에 저장합니다.
2. JSON document event를 SQS에 발행합니다.
3. Worker가 message 하나를 받아 S3 object를 다운로드하고 DynamoDB에
   processing state를 기록합니다.
4. Processing이 terminal 상태일 때만 SQS message를 acknowledge합니다. 즉
   state row가 새로 생성되었거나, DynamoDB가 이미 존재한다고 알려준 경우입니다.
5. S3, SQS, DynamoDB의 transient failure는 message를 삭제하지 않고
   visibility를 바꿔 retry 가능하게 둡니다.

DynamoDB item은 다음 형태를 사용합니다.

| Attribute | Example | 이유 |
|---|---|---|
| `pk` | `TENANT#tenant-alpha` | Tenant별 document state를 묶습니다. |
| `sk` | `DOCUMENT#doc-1001#PROCESSING` | Document 하나당 processing record 하나를 둡니다. |
| `object_key` | `tenants/tenant-alpha/documents/doc-1001/contract-1001.txt` | State가 어떤 S3 object를 처리했는지 보여줍니다. |
| `idempotency_key` | `tenant-alpha/doc-1001/process` | 중복 작업을 log와 state에서 식별합니다. |
| `status` | `PROCESSED` | Event가 terminal success에 도달했음을 나타냅니다. |
| `bytes_read` | `14` | Worker가 state 기록 전에 S3 body를 읽었음을 증명합니다. |

## Architecture

![S3 SQS DynamoDB document workflow architecture](../../docs/images/readme-diagrams/s3-sqs-dynamodb-document-workflow-architecture.png)

핵심은 ownership 분리입니다.

1. `DocumentWorkflow`는 application contract만 소유합니다. Object key, SQS
   event, DynamoDB state key, condition expression, ack/retry 결정을 다룹니다.
2. S3, SQS, DynamoDB client는 caller-owned AWS SDK v2 client입니다. Local
   smoke test에서는 `testcontainers/floci`가 이 client 설정을 제공합니다.
3. Floci는 local endpoint와 test credential만 제공합니다. Production IAM,
   encryption, alarm, DLQ/redrive policy, deployment topology는 workshop code
   밖의 책임입니다.

## Processing Sequence

![S3 SQS DynamoDB document workflow sequence](../../docs/images/readme-diagrams/s3-sqs-dynamodb-document-workflow-sequence.png)

`ProcessOnce`는 보이는 SQS message를 최대 하나만 처리합니다. 그래서 예제가
작지만, 실제 delivery rule은 그대로 드러납니다.

| Result | Worker behavior | SQS acknowledgement |
|---|---|---|
| New document | S3 object를 다운로드하고 `attribute_not_exists(pk) AND attribute_not_exists(sk)` 조건으로 DynamoDB processing row를 생성합니다. | SQS message를 삭제합니다. |
| Duplicate event | DynamoDB `ConditionalCheckFailedException`을 `ErrConditionalConflict`로 감쌉니다. | 이미 처리된 terminal work이므로 SQS message를 삭제합니다. |
| Transient S3/DynamoDB failure | Terminal state를 기록하지 않습니다. | `ChangeMessageVisibility`로 같은 message가 retry되게 둡니다. |
| Invalid message body | Worker state path를 호출하지 않습니다. | visibility를 바꿔 inspection/retry policy에 맡깁니다. |

## What It Demonstrates

- Tenant/document 단위의 안전한 S3 object key와 metadata.
- SQS `DocumentEvent` JSON body와 `content-type`, `event-type`,
  `idempotency-key` message attributes.
- Worker path에서 S3 `GetObject` response body를 닫는 책임.
- DynamoDB conditional `PutItem`을 idempotent processing gate로 쓰는 방식.
- Duplicate processing을 retry storm이 아니라 terminal acknowledgement로
  처리하는 방식.
- Docker 없이 검증하는 deterministic fake-client tests와 opt-in multi-service
  Floci smoke test.

## Run

Local preview를 출력합니다.

```bash
go run ./examples/s3-sqs-dynamodb-document-workflow
```

출력은 JSON이며 AWS service에 접속하지 않습니다.

```json
{
  "bucket": "documents",
  "queue_name": "document-events",
  "table": "document_processing",
  "object_prefix": "tenants/<tenant_id>/documents/<document_id>/<file_name>",
  "state_key": "pk=TENANT#<tenant_id>, sk=DOCUMENT#<document_id>#PROCESSING"
}
```

실제 preview에는 operations, idempotency rules, prerequisite examples,
smoke-test command도 함께 포함됩니다.

## Test

Deterministic test를 실행합니다.

```bash
go test -count=1 ./examples/s3-sqs-dynamodb-document-workflow/...
go test -race -count=1 ./examples/s3-sqs-dynamodb-document-workflow/...
```

Targeted tests는 다음을 증명합니다.

- `Submit`은 SQS event를 보내기 전에 S3에 document를 저장합니다.
- SQS event에는 object key, content type, idempotency key가 들어갑니다.
- `ProcessOnce`는 S3 object를 다운로드하고 response body를 닫습니다.
- DynamoDB `PutItem`은 `attribute_not_exists(pk) AND attribute_not_exists(sk)`
  조건을 사용합니다.
- Duplicate conditional conflict는 이미 처리된 work로 acknowledge됩니다.
- Transient DynamoDB error는 visibility를 바꾸고 message를 삭제하지 않습니다.
- 위조된 SQS event body는 object key나 idempotency key를 다른 값으로 돌릴 수
  없습니다.
- Canceled context는 external client call 전에 중단됩니다.

## Optional Floci Smoke Test

Floci-backed smoke test는 S3, SQS, DynamoDB를 켠 Docker-backed local
AWS-compatible container 하나를 시작합니다. Docker가 있을 때 serial로 실행하세요.

```bash
BLUETAPE_DOCUMENT_WORKFLOW_SMOKE=1 go test -run TestFlociSmoke -count=1 ./examples/s3-sqs-dynamodb-document-workflow/...
```

Smoke test는 bucket, queue, DynamoDB table을 만들고 sample document를 제출한
뒤 message를 성공 처리합니다. 그 다음 같은 document를 다시 제출해서 DynamoDB
conditional write가 duplicate event를 안전한 acknowledgement로 바꾸는지
확인합니다.

실제 AWS credential은 사용하지 않습니다. Real deployment에는 IAM,
encryption, DLQ/redrive policy, observability, cleanup automation이 이 예제
밖에서 필요합니다.
