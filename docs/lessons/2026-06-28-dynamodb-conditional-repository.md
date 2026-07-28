# DynamoDB conditional repository 예제

Issue: #61

## 결정

DynamoDB를 S3 또는 SQS workflow orchestration과 조합하기 전에 focused DynamoDB repository
example로 conditional write를 설명한다. 예제는 application surface를 작게 유지한다. catalog
item이 없을 때 생성하고, caller가 current version을 가진 경우에만 name을 update하며,
tenant partition을 query한다.

## 이유

DynamoDB conditional write failure는 일반적인 infrastructure failure가 아니다. 보통 row가
이미 있거나 caller가 stale하다는 business predicate 실패를 뜻한다. 모든 caller가 key
attribute, condition expression, conflict handling을 조립하면 같은 consistency rule이
반복되고 결국 divergence가 생긴다.

따라서 예제는 repository가 다음을 소유하게 한다.

- `TENANT#...` / `ITEM#...` key construction.
- create-if-absent 및 expected-version condition expression.
- application conflict signal인 `ErrConditionalConflict`.
- diagnostic을 위한 typed AWS SDK error preservation.
- `pk` equality와 `begins_with(sk, ITEM#)`를 사용하는 tenant query shape.

## 검증 형태

- deterministic fake-client test는 정확한 `PutItem`, `UpdateItem`, `Query` expression을
  assert한다.
- conflict test는 wrapping 뒤에도 AWS SDK `ConditionalCheckFailedException`을 `errors.As`로
  찾을 수 있음을 assert한다.
- cancellation 및 validation test는 caller-owned failure가 DynamoDB client boundary 전에
  멈춤을 증명한다.
- README diagram은 ownership boundary를 runtime success/conflict/query sequence와 분리해
  설명한다.
- Floci smoke test는 Docker-backed DynamoDB emulation을 시작하고 다른 container suite와
  serial로 실행되어야 하므로 opt-in으로 남긴다.
