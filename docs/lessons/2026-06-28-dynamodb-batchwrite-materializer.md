# DynamoDB batch write materializer example

Issue: #116

## Decision

Use a local document-index materializer to teach the `dynamodb/batchwrite`
boundary before larger AWS Floci workflow examples. The example maps domain
events to AWS SDK v2 `types.WriteRequest` values, then delegates chunking and
`UnprocessedItems` retry to `batchwrite.WriteAll`.

## Why

DynamoDB batch writes have a small but important protocol: at most 25 requests
per call, partial success via `UnprocessedItems`, and different handling for
retry exhaustion, typed service errors, and caller cancellation. Keeping that
protocol in every worker would make examples noisy and error-prone.

The example therefore keeps item-shape ownership in the application while using
bluetape-go for the DynamoDB batch mechanics. That makes the domain keys,
attribute names, retry budget, and production handoff policy reviewable without
reimplementing the helper.

## Verification shape

- Deterministic fake-client tests assert 30 events become `25 + 5` DynamoDB
  calls.
- Retry tests assert only returned `UnprocessedItems` are submitted again.
- Exhaustion tests assert `ErrRetryExhausted` while preserving
  `batchwrite.UnprocessedItemsError`.
- Cancellation and typed AWS service error tests assert the caller can still
  distinguish ownership and infrastructure failures.
- README diagrams explain architecture ownership separately from retry
  sequencing.
