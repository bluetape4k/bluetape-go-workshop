# Code review: issue #61 DynamoDB conditional repository

## Scope

- New runnable example: `examples/dynamodb-conditional-repository`
- New README diagrams for repository ownership and conditional-write sequence
- Root README navigation and focused run/test instructions
- Lesson note for DynamoDB conditional write boundary selection

## Findings

No P0/P1 findings in the local review pass.

## Checks

- Repository API keeps callers away from DynamoDB expression assembly.
- `CreateIfAbsent` uses `attribute_not_exists(pk)` and
  `attribute_not_exists(sk)`.
- `UpdateName` checks the caller's expected `version` and returns the updated
  item.
- Conditional conflicts become `ErrConditionalConflict` while preserving the
  typed AWS SDK error.
- `QueryTenant` uses the tenant partition key and `ITEM#` sort-key prefix.
- Context cancellation is checked before the client call boundary.
- Floci-backed DynamoDB smoke coverage stays opt-in and serial-friendly.
- README diagrams render as PNG and keep ownership boundaries separate from the
  runtime success/conflict/query sequence.

## Validation evidence

- `go run ./examples/dynamodb-conditional-repository`
- `go test -count=1 ./examples/dynamodb-conditional-repository/...`
- `go test -race -count=1 ./examples/dynamodb-conditional-repository/...`
- `make ci`
- SVG XML parse plus CairoSVG render for
  `dynamodb-conditional-repository-architecture.svg` and
  `dynamodb-conditional-repository-sequence.svg`
- Local link/image existence check for root README files and the new example
  README pair
- `git diff --check`

## Residual risk

The example does not model S3/SQS document orchestration, provisioned capacity
tuning, global secondary indexes, IAM policy, or production alarms. Those are
downstream workflow concerns and are deliberately documented as out of scope so
this example stays focused on repository-level conditional consistency.

The optional diagram geometry and endpoint audit helper scripts referenced by
the local diagram skill were not present at the installed path, so the diagram
gate used XML parsing, CairoSVG rendering, marker inspection, and full-size PNG
inspection instead.

## P0/P1 Gate

P0=0 P1=0
