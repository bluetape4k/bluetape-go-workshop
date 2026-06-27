# Code review: issue #116 DynamoDB batch write materializer

## Scope

- New runnable example: `examples/dynamodb-batchwrite-materializer`
- New README diagrams for architecture and retry sequencing
- Root README navigation and focused run instructions
- Lesson note for DynamoDB batch-write boundary selection

## Findings

No P0/P1 findings in the local review pass.

## Checks

- Application code owns document event validation and DynamoDB item shape.
- `batchwrite.WriteAll` owns 25-item chunking and retries only returned
  `UnprocessedItems`.
- Retry exhaustion is separated from typed AWS SDK service errors and still
  preserves `batchwrite.UnprocessedItemsError`.
- Context cancellation is propagated through the retry path.
- Floci-backed DynamoDB smoke coverage stays opt-in and serial-friendly.
- README diagrams render as PNG and keep ownership boundaries separate from the
  runtime retry sequence.

## Validation evidence

- `go run ./examples/dynamodb-batchwrite-materializer`
- `go test -count=1 ./examples/dynamodb-batchwrite-materializer/...`
- `go test -race -count=1 ./examples/dynamodb-batchwrite-materializer/...`
- `make ci`
- SVG XML parse plus CairoSVG render for
  `dynamodb-batchwrite-materializer-architecture.svg` and
  `dynamodb-batchwrite-materializer-retry-sequence.svg`
- Local link/image existence check for root README files and the new example
  README pair
- `git diff --check`

## Residual risk

The example does not model S3/SQS ingestion, provisioned capacity tuning,
dead-letter replay, IAM policy, or production alarms. Those are downstream
workflow concerns and are deliberately documented as infrastructure handoff
points so this example stays focused on `dynamodb/batchwrite`.

The optional diagram geometry and endpoint audit helper scripts referenced by
the local diagram skill were not present at the installed path, so the diagram
gate used XML parsing, CairoSVG rendering, marker inspection, and full-size PNG
inspection instead.

## P0/P1 Gate

P0=0 P1=0
