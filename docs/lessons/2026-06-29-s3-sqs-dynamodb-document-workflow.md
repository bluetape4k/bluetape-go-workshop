# S3-SQS-DynamoDB Document Workflow Lesson

Issue #66 is the AWS/Floci integration example that composes the earlier S3,
SQS, and DynamoDB lessons. Keep it application-shaped: the workflow owns object
key conventions, event shape, idempotency state, and ack/retry decisions, while
AWS SDK clients, Floci endpoints, credentials, IAM, encryption, and DLQ policy
remain caller-owned.

The reader contract is easier to follow when the example has two diagrams:

- An architecture view that separates the application boundary,
  workflow-owned contracts, and caller-owned AWS/Floci resources.
- A sequence view that shows submit, success, duplicate, and transient retry
  outcomes in time order.

Tests should prove more than the happy path. For this workflow, the minimum
deterministic suite covers S3 put before SQS send, S3 body close on processing,
DynamoDB conditional write shape, duplicate ack, transient retry visibility,
context cancellation, unsafe key rejection, and forged SQS event consistency.

The opt-in Floci smoke test should start one local AWS-compatible container with
S3, SQS, and DynamoDB enabled, then run serially. It proves SDK request
compatibility without requiring real AWS credentials or account state.

Diagram QA lesson: inspect full-size PNGs after CairoSVG render, not only SVG.
The first architecture pass had a connector crossing the retry-visible note and
AWS card labels with tight margins; both were visible only in the rendered PNG.
