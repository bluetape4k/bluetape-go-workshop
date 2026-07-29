# Code review: issue #116 DynamoDB batch write materializer

## 범위

- 새 runnable example: `examples/dynamodb-batchwrite-materializer`
- architecture와 retry sequencing을 위한 새 README diagram
- root README navigation과 focused run instruction
- DynamoDB batch-write boundary selection lesson note

## 발견 사항

local review pass에서 P0/P1 finding은 없다.

## 점검

- application code는 document event validation과 DynamoDB item shape를 소유한다.
- `batchwrite.WriteAll`은 25-item chunking을 소유하고, 반환된 `UnprocessedItems`만 retry한다.
- retry exhaustion은 typed AWS SDK service error와 분리되며
  `batchwrite.UnprocessedItemsError`를 계속 보존한다.
- context cancellation은 retry path를 통해 전파된다.
- Floci-backed DynamoDB smoke coverage는 opt-in이고 serial-friendly로 남는다.
- README diagram은 PNG로 render되며 ownership boundary와 runtime retry sequence를 분리한다.

## 검증 Evidence

- `go run ./examples/dynamodb-batchwrite-materializer`
- `go test -count=1 ./examples/dynamodb-batchwrite-materializer/...`
- `go test -race -count=1 ./examples/dynamodb-batchwrite-materializer/...`
- `make ci`
- SVG XML parse plus CairoSVG render for
  `dynamodb-batchwrite-materializer-architecture.svg` and
  `dynamodb-batchwrite-materializer-retry-sequence.svg`
- root README files와 새 example README pair에 대한 local link/image existence check
- `git diff --check`

## 잔여 Risk

예제는 S3/SQS ingestion, provisioned capacity tuning, dead-letter replay, IAM policy,
production alarm을 모델링하지 않는다. 이들은 downstream workflow concern이며, 예제가
`dynamodb/batchwrite`에 집중하도록 infrastructure handoff point로 문서화되어 있다.

local diagram skill이 참조한 optional diagram geometry 및 endpoint audit helper script가 설치된
경로에 없었기 때문에 diagram gate는 XML parsing, CairoSVG rendering, marker inspection,
full-size PNG inspection을 사용했다.

## P0/P1 Gate

P0=0 P1=0
