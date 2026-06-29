# Issue #66 S3-SQS-DynamoDB Code Review

Scope: `examples/s3-sqs-dynamodb-document-workflow`, root README catalog entries,
README diagram assets, and the #66 lessons/review artifacts.

Baseline: local branch `feat/issue-66-s3-sqs-dynamodb` against `origin/develop`.

## Findings

P0=0 P1=0

No blocking findings remain.

## Evidence

- `Submit` derives the S3 object key and idempotency key before S3/SQS calls:
  `examples/s3-sqs-dynamodb-document-workflow/internal/documentworkflow/workflow.go:199`.
- `ProcessOnce` acknowledges only terminal outcomes and changes visibility for
  retryable failures: `examples/s3-sqs-dynamodb-document-workflow/internal/documentworkflow/workflow.go:280`.
- `decodeEvent` rejects forged SQS bodies whose object key or idempotency key
  does not match tenant/document/file identity:
  `examples/s3-sqs-dynamodb-document-workflow/internal/documentworkflow/workflow.go:366`.
- Deterministic tests cover S3 persistence before SQS send, S3 body close,
  DynamoDB conditional write shape, duplicate ack, retry visibility, forged
  event rejection, cancellation, and unsafe key rejection:
  `examples/s3-sqs-dynamodb-document-workflow/internal/documentworkflow/workflow_test.go:33`.
- The opt-in smoke test starts one Floci container with S3, SQS, and DynamoDB,
  then verifies success and duplicate processing:
  `examples/s3-sqs-dynamodb-document-workflow/internal/documentworkflow/smoke_test.go:19`.
- README files include prerequisite links, architecture and sequence diagrams,
  run commands, deterministic test commands, and optional Floci smoke command:
  `examples/s3-sqs-dynamodb-document-workflow/README.md:8`.

## Validation

- `go test -count=1 ./examples/s3-sqs-dynamodb-document-workflow/...`
- `go test -race -count=1 ./examples/s3-sqs-dynamodb-document-workflow/...`
- `go run ./examples/s3-sqs-dynamodb-document-workflow`
- `xmllint --noout docs/images/readme-diagrams/s3-sqs-dynamodb-document-workflow-architecture.svg docs/images/readme-diagrams/s3-sqs-dynamodb-document-workflow-sequence.svg`
- `~/.local/bin/cairosvg ... -s 2` for both SVG diagrams
- Full-size PNG inspection for both diagrams
- Contact sheet inspection: `/tmp/s3-sqs-dynamodb-document-workflow-contact.png`
- `rg -n 'context-stroke|markerUnits="strokeWidth"' docs/images/readme-diagrams/s3-sqs-dynamodb-document-workflow-*.svg`
- `git diff --check`

## Validation Gaps

The diagram helper scripts referenced by the installed diagram skill are not
present in this environment:

- `/Users/debop/.codex/skills/bluetape4k-diagram/references/diagram-geometry-audit.py`
- `/Users/debop/.codex/skills/bluetape4k-diagram/references/diagram-endpoint-audit.py`

This was mitigated with XML validation, marker/icon scans, CairoSVG rendering,
full-size PNG inspection, and contact sheet inspection.
