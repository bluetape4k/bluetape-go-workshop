# s3-floci-storage

[English](README.md) | [한국어](README.ko.md)

AWS SDK for Go v2와 Floci-backed smoke test를 사용하는 local S3 storage 예제입니다.

이 예제는 작은 receipt storage boundary를 모델링합니다. Application code는 business
command를 계속 소유하지만, storage component는 S3 object key 규칙, metadata contract,
body close 규칙, missing-object mapping, presigned download URL shape를 소유합니다.

예제는 다섯 가지 operation을 다룹니다.

- `Upload`는 receipt object를 content type, size, metadata와 함께 저장합니다.
- `Download`는 object body를 읽고 S3 response body를 닫습니다.
- `ListTenantReceipts`는 한 tenant prefix를 list합니다.
- `Delete`는 receipt object 하나를 삭제합니다.
- `PresignDownload`는 bounded TTL을 가진 caller-facing GET URL을 만듭니다.

실제 cloud credential은 필요하지 않습니다. 일반 test는 deterministic fake를 사용하고,
optional smoke test만 bluetape-go의 `testcontainers/floci` fixture로 Floci를 시작합니다.

## 시나리오

Checkout service는 생성된 receipt document를 object storage에 씁니다. 이후 support
tool은 한 tenant의 receipt를 list하고, 선택한 receipt를 download하고, 수정된 receipt를
delete하거나, 신뢰된 caller에게 짧게 살아 있는 download URL을 전달해야 합니다.

Store는 하나의 bucket과 tenant-shaped object key를 사용합니다.

| Field | 예 | 이유 |
|---|---|---|
| Bucket | `tenant-receipts` | Deployment가 소유하는 storage 위치입니다. |
| Tenant prefix | `tenants/tenant-alpha/receipts/` | List operation을 tenant 범위로 제한합니다. |
| Object key | `tenants/tenant-alpha/receipts/receipt-1001/invoice-1001.txt` | Tenant, receipt, file name이 S3에서 보이게 합니다. |
| Metadata | `tenant_id`, `receipt_id`, `source` | 진단 metadata를 object 가까이에 둡니다. |
| Content type | `text/plain; charset=utf-8` | Extension을 먼저 보고, 필요하면 payload sample로 판단합니다. |
| Presign TTL | 기본 `15m` | Download link를 의도적으로 짧게 유지합니다. |

## Architecture

![S3 Floci storage architecture](../../docs/images/readme-diagrams/s3-floci-storage-architecture.png)

Architecture는 ownership boundary를 작게 유지합니다.

1. Application은 어떤 receipt operation이 필요한지 결정합니다.
2. `ReceiptStore`는 safe object key와 S3 request를 만듭니다.
3. AWS SDK v2 client는 caller-owned로 유지되므로 production code도 local 예제와 같은
   request/response type을 사용할 수 있습니다.
4. Floci는 smoke test를 위한 local endpoint, test credential, path-style S3 behavior를
   제공합니다.

## Operation Sequence

![S3 Floci storage sequence](../../docs/images/readme-diagrams/s3-floci-storage-sequence.png)

Runtime contract는 의도적으로 작습니다.

| Operation | S3 request | 예제 책임 |
|---|---|---|
| Upload | `PutObject` | Key, body, content type, content length, metadata를 설정합니다. |
| Download | `GetObject` | Response body를 읽고 닫습니다. |
| List | `ListObjectsV2` | Bucket-wide scan 대신 tenant prefix를 사용합니다. |
| Delete | `DeleteObject` | Safe object key 하나만 삭제합니다. |
| Presign | `PresignGetObject` | TTL이 있는 caller-owned GET URL을 만듭니다. |

`NoSuchKey`는 `ErrObjectNotFound`로 변환합니다. 그래서 application code는 missing
receipt와 network, credential, endpoint, permission failure를 분리해서 다룰 수 있습니다.

## Local Floci vs Real AWS

| Concern | Local Floci smoke test | Real AWS deployment |
|---|---|---|
| Endpoint | `testcontainers/floci` detail에서 가져옵니다. | 일반 AWS SDK config에서 가져옵니다. |
| Credentials | Floci container의 static test credential을 씁니다. | IAM role, profile, workload identity를 씁니다. |
| Addressing | `UsePathStyle = true`가 필요합니다. | 보통 virtual-hosted style이며 endpoint가 요구할 때만 path style을 씁니다. |
| Bucket lifecycle | Test가 임시 bucket을 만들고 삭제합니다. | Infrastructure가 bucket creation, policy, encryption, lifecycle rule을 소유합니다. |
| Scope | Upload/download/list/delete/presign behavior입니다. | Monitoring, KMS, replication, object lock, access policy는 별도 관심사입니다. |

## 보여주는 것

- AWS SDK 전체를 감싸지 않는 작은 S3 storage abstraction.
- Safe tenant-shaped object key.
- Upload 시 metadata와 content type 설정.
- Download 시 response body close.
- Tenant-prefix listing.
- Missing-object error mapping.
- Bounded TTL을 가진 presigned GET URL.
- Deterministic fake-client test와 opt-in Floci S3 smoke test.

## 실행

Local preview를 출력합니다.

```bash
go run ./examples/s3-floci-storage
```

Output은 JSON이며 S3에 접속하지 않습니다.

```json
{
  "bucket": "tenant-receipts",
  "object_prefix": "tenants/<tenant_id>/receipts/<receipt_id>/<file_name>"
}
```

실제 output에는 지원 operation, local contract, smoke-test command도 포함됩니다.

## 테스트

Deterministic test를 실행합니다.

```bash
go test -count=1 ./examples/s3-floci-storage/...
go test -race -count=1 ./examples/s3-floci-storage/...
```

Targeted test는 다음을 증명합니다.

- upload가 safe key, metadata, content length, content type을 mapping합니다.
- download가 response body를 읽고 닫습니다.
- `NoSuchKey`는 `ErrObjectNotFound`가 됩니다.
- list는 tenant prefix를 사용합니다.
- delete는 object key 하나를 대상으로 합니다.
- presign은 GET과 configured TTL을 사용합니다.
- canceled context는 S3 호출 전에 반환됩니다.

## Optional Floci Smoke Test

Floci-backed smoke test는 opt-in입니다. Docker-backed AWS service emulation을
시작하므로 다른 container-backed suite와 serial로 실행하는 것이 좋습니다.

```bash
BLUETAPE_S3_FLOCI_STORAGE_SMOKE=1 go test -run TestFlociSmoke -count=1 ./examples/s3-floci-storage/...
```

Smoke test는 S3를 활성화한 Floci를 시작하고, 임시 bucket을 만들고, receipt 하나를
upload/download/list/presign/delete한 뒤 이후 download가 `ErrObjectNotFound`로
mapping되는지 확인합니다.

## Boundary Notes

- Bucket provisioning, IAM, encryption, lifecycle policy, alarm은 infrastructure code에
  둡니다.
- SQS/DynamoDB document workflow orchestration은 이 예제에 넣지 않습니다. 이후
  integration issue에서 다루는 편이 낫습니다.
- Floci와 다른 local endpoint에는 path-style S3 addressing을 사용합니다. 실제 AWS에는
  endpoint가 요구할 때만 이 설정을 가져갑니다.
