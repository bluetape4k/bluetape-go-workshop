# dynamodb-conditional-repository

[English](README.md) | [한국어](README.ko.md)

Conditional write를 위한 local DynamoDB repository 예제입니다.

이 예제는 tenant별 catalog item을 하나의 DynamoDB table에 저장하는 작은 repository를
모델링합니다. 크기는 작지만, application code로 쉽게 새어 나오는 DynamoDB의 핵심
규칙을 다룹니다. 바로 conditional write expression과 conditional conflict의 의미입니다.

예제는 세 가지 repository operation을 다룹니다.

- `CreateIfAbsent`는 `pk` / `sk` 쌍이 아직 없을 때만 catalog item을 씁니다.
- `UpdateName`은 저장된 `version`이 caller의 expected version과 같을 때만 optimistic
  update를 수행합니다.
- `QueryTenant`는 `begins_with(sk, ITEM#)` key condition으로 tenant partition을 읽습니다.

S3, SQS와 일부러 분리했습니다. Conditional write는 repository consistency lesson입니다.
더 큰 document workflow는 뒤의 integration 예제에서 다루는 편이 낫습니다.

## 시나리오

Tenant catalog worker는 upstream product system에서 검수된 item 변경을 받습니다.
Worker는 새 item을 만들고, duplicate create를 거절하고, caller가 최신 version을 알고
있을 때만 item을 update하고, 한 tenant의 모든 item을 query해야 합니다.

Table은 간단한 single-table key shape를 사용합니다.

| Attribute | 예 | 이유 |
|---|---|---|
| `pk` | `TENANT#tenant-alpha` | 예제에서 tenant catalog를 하나의 partition에 둡니다. |
| `sk` | `ITEM#sku-1001` | `begins_with`로 item row를 query할 수 있게 합니다. |
| `tenant_id` | `tenant-alpha` | Domain identifier를 보존합니다. |
| `sku` | `sku-1001` | Item identifier를 보존합니다. |
| `name` | `Road bike` | Update 가능한 attribute를 보여줍니다. |
| `version` | `1` | Optimistic write conflict를 판단합니다. |
| `updated_by` | `seed` | Update와 함께 바뀌는 audit metadata를 보여줍니다. |

## Architecture

![DynamoDB conditional repository architecture](../../docs/images/readme-diagrams/dynamodb-conditional-repository-architecture.png)

Architecture는 consistency boundary를 명시적으로 나눕니다.

1. Application service는 catalog item을 create할지 update할지 결정합니다.
2. Repository는 DynamoDB key mapping, condition expression, typed conflict
   translation을 소유합니다.
3. DynamoDB는 create-if-absent와 expected-version predicate를 강제합니다.
4. Caller는 business conflict를 `ErrConditionalConflict`로 보고, 원래 AWS SDK
   `ConditionalCheckFailedException`도 `errors.As`로 계속 찾을 수 있습니다.

## Conditional Write Sequence

![DynamoDB conditional repository sequence](../../docs/images/readme-diagrams/dynamodb-conditional-repository-sequence.png)

Repository는 conditional conflict를 transport error나 AWS service error와 다르게
취급합니다.

| Operation | Condition | Conflict 의미 |
|---|---|---|
| Create item | `attribute_not_exists(pk) AND attribute_not_exists(sk)` | Item이 이미 있으므로 caller가 duplicate를 만들면 안 됩니다. |
| Update item | `version = expectedVersion` | Caller가 stale 상태이므로 reload 후 retry해야 합니다. |
| Query tenant | `pk = TENANT#... AND begins_with(sk, ITEM#)` | Conflict path는 없고 read contract만 있습니다. |

Repository는 DynamoDB를 호출하기 전에 `ctx.Err()`를 확인합니다. 그래서 caller가 소유한
cancellation이 DynamoDB error로 바뀌지 않습니다. Test는 wrapping 이후에도 typed
conditional error가 보존되는지 검증합니다.

## 보여주는 것

- Key design을 ORM 뒤에 숨기지 않는 AWS SDK v2 `PutItem`, `UpdateItem`, `Query`
  request shape.
- `attribute_not_exists`를 사용한 create-if-absent write.
- 명시적인 version 비교를 사용한 optimistic update.
- Sort-key prefix로 tenant partition을 query하는 방식.
- Application-level conflict signal인 `ErrConditionalConflict`.
- `errors.As`로 AWS SDK typed error를 보존하는 wrapping.
- Deterministic fake-client test와 opt-in Floci DynamoDB smoke test.

## 실행

Local preview를 출력합니다.

```bash
go run ./examples/dynamodb-conditional-repository
```

Output은 JSON이며 DynamoDB에 접속하지 않습니다.

```json
{
  "table": "catalog_items",
  "item_count": 3,
  "partition_key": "TENANT#<tenant_id>",
  "sort_key_prefix": "ITEM#<sku>"
}
```

실제 output에는 repository operation, conflict rule, optional smoke-test command도
포함됩니다.

## 테스트

Deterministic test를 실행합니다.

```bash
go test -count=1 ./examples/dynamodb-conditional-repository/...
go test -race -count=1 ./examples/dynamodb-conditional-repository/...
```

Targeted test는 다음을 증명합니다.

- create call에 `attribute_not_exists` condition이 들어갑니다.
- duplicate create는 `ErrConditionalConflict`가 됩니다.
- optimistic update는 caller의 expected version을 사용하고 updated item을 반환합니다.
- stale update는 `ErrConditionalConflict`가 됩니다.
- tenant query는 partition key와 item sort-key prefix를 사용합니다.
- canceled context는 DynamoDB 호출 전에 반환됩니다.
- invalid item shape는 client boundary 전에 실패합니다.

## Optional Floci Smoke Test

Floci-backed smoke test는 opt-in입니다. Docker-backed AWS service emulation을
시작하므로 다른 container-backed suite와 serial로 실행하는 것이 좋습니다.

```bash
BLUETAPE_DYNAMODB_CONDITIONAL_SMOKE=1 go test -run TestFlociSmoke -count=1 ./examples/dynamodb-conditional-repository/...
```

Smoke test는 DynamoDB table을 만들고, item 하나를 쓰고, duplicate create conflict를
확인하고, item을 version 1에서 version 2로 update하고, stale version conflict를 확인한
뒤 tenant partition을 query합니다.

## Boundary Notes

- Key shape와 condition expression은 repository에 둡니다. Caller는 operation을
  선택해야지 DynamoDB expression을 조립하면 안 됩니다.
- Conditional conflict는 generic infrastructure failure가 아니라 business consistency
  conflict로 취급합니다.
- Capacity, validation, permission, transport 문제를 분리해서 진단할 수 있도록 AWS SDK
  typed error를 보존합니다.
- 이 예제는 repository consistency에 집중합니다. S3/SQS document workflow orchestration은
  나중에 이 boundary를 조합하는 쪽이 낫습니다.
