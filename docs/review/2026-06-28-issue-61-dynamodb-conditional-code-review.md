# Code review: issue #61 DynamoDB conditional repository

## 범위

- 새 runnable example: `examples/dynamodb-conditional-repository`
- repository ownership과 conditional-write sequence를 위한 새 README diagram
- root README navigation과 focused run/test instruction
- DynamoDB conditional write boundary selection lesson note

## 발견 사항

local review pass에서 P0/P1 finding은 없다.

## 점검

- repository API는 caller가 DynamoDB expression assembly를 직접 다루지 않게 한다.
- `CreateIfAbsent`는 `attribute_not_exists(pk)`와 `attribute_not_exists(sk)`를 사용한다.
- `UpdateName`은 caller의 expected `version`을 확인하고 updated item을 반환한다.
- conditional conflict는 typed AWS SDK error를 보존하면서 `ErrConditionalConflict`가 된다.
- `QueryTenant`는 tenant partition key와 `ITEM#` sort-key prefix를 사용한다.
- context cancellation은 client call boundary 전에 확인된다.
- Floci-backed DynamoDB smoke coverage는 opt-in이고 serial-friendly로 남는다.
- README diagram은 PNG로 render되며 ownership boundary를 runtime success/conflict/query
  sequence와 분리한다.

## 검증 Evidence

- `go run ./examples/dynamodb-conditional-repository`
- `go test -count=1 ./examples/dynamodb-conditional-repository/...`
- `go test -race -count=1 ./examples/dynamodb-conditional-repository/...`
- `make ci`
- SVG XML parse plus CairoSVG render for
  `dynamodb-conditional-repository-architecture.svg` and
  `dynamodb-conditional-repository-sequence.svg`
- root README files와 새 example README pair에 대한 local link/image existence check
- `git diff --check`

## 잔여 Risk

예제는 S3/SQS document orchestration, provisioned capacity tuning, global secondary index,
IAM policy, production alarm을 모델링하지 않는다. 이들은 downstream workflow concern으로
문서화되어 있으며, 예제는 repository-level conditional consistency에 집중한다.

local diagram skill이 참조한 optional diagram geometry 및 endpoint audit helper script가 설치된
경로에 없었기 때문에 diagram gate는 XML parsing, CairoSVG rendering, marker inspection,
full-size PNG inspection을 사용했다.

## P0/P1 Gate

P0=0 P1=0
