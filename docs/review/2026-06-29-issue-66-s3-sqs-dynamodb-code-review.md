# Issue #66 S3-SQS-DynamoDB Code Review

## 범위

`examples/s3-sqs-dynamodb-document-workflow`, root README catalog entry, README diagram
asset, #66 lesson/review artifact를 검토했다.

Baseline: local branch `feat/issue-66-s3-sqs-dynamodb` against `origin/develop`.

## 발견 사항

P0=0 P1=0

blocking finding은 남아 있지 않다.

## 검증 자료

- `Submit`은 S3/SQS call 전에 S3 object key와 idempotency key를 파생한다:
  `examples/s3-sqs-dynamodb-document-workflow/internal/documentworkflow/workflow.go:199`.
- `ProcessOnce`는 terminal outcome만 acknowledge하고 retryable failure에서는 visibility를
  변경한다:
  `examples/s3-sqs-dynamodb-document-workflow/internal/documentworkflow/workflow.go:280`.
- `decodeEvent`는 object key 또는 idempotency key가 tenant/document/file identity와 맞지 않는
  forged SQS body를 거부한다:
  `examples/s3-sqs-dynamodb-document-workflow/internal/documentworkflow/workflow.go:366`.
- deterministic test는 SQS send 전 S3 persistence, S3 body close, DynamoDB conditional write
  shape, duplicate ack, retry visibility, forged event rejection, cancellation, unsafe key rejection을
  다룬다:
  `examples/s3-sqs-dynamodb-document-workflow/internal/documentworkflow/workflow_test.go:33`.
- opt-in smoke test는 S3, SQS, DynamoDB가 활성화된 Floci container 하나를 시작한 뒤 success와
  duplicate processing을 검증한다:
  `examples/s3-sqs-dynamodb-document-workflow/internal/documentworkflow/smoke_test.go:19`.
- README file은 prerequisite link, architecture/sequence diagram, run command,
  deterministic test command, optional Floci smoke command를 포함한다:
  `examples/s3-sqs-dynamodb-document-workflow/README.md:8`.

## 검증

- `go test -count=1 ./examples/s3-sqs-dynamodb-document-workflow/...`
- `go test -race -count=1 ./examples/s3-sqs-dynamodb-document-workflow/...`
- `go run ./examples/s3-sqs-dynamodb-document-workflow`
- `xmllint --noout docs/images/readme-diagrams/s3-sqs-dynamodb-document-workflow-architecture.svg docs/images/readme-diagrams/s3-sqs-dynamodb-document-workflow-sequence.svg`
- both SVG diagram에 대해 `~/.local/bin/cairosvg ... -s 2`
- both diagram에 대한 full-size PNG inspection
- contact sheet inspection: `/tmp/s3-sqs-dynamodb-document-workflow-contact.png`
- `rg -n 'context-stroke|markerUnits="strokeWidth"' docs/images/readme-diagrams/s3-sqs-dynamodb-document-workflow-*.svg`
- `git diff --check`

## 검증 Gap

installed diagram skill이 참조한 diagram helper script는 이 environment에 없다.

- `/Users/debop/.codex/skills/bluetape4k-diagram/references/diagram-geometry-audit.py`
- `/Users/debop/.codex/skills/bluetape4k-diagram/references/diagram-endpoint-audit.py`

이를 XML validation, marker/icon scan, CairoSVG rendering, full-size PNG inspection,
contact sheet inspection으로 보완했다.
