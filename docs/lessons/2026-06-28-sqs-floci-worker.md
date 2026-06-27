# SQS Floci Worker Example Lesson

## Context

Issue #60 adds the SQS worker step in the v0.7.0 AWS/Floci workshop track. The
reader needs to understand the application boundary around SQS, not a broad
queue framework: message shape, idempotency metadata, receive policy,
acknowledgement, and retry visibility.

## Decision

The example keeps AWS SDK v2 clients caller-owned and exposes a small
`TaskQueue` boundary with `Enqueue` and `PollOnce`. `PollOnce` receives one
message, calls an application handler, deletes only after success, and changes
visibility after failure so SQS can redeliver the same idempotent task.

## Rejected

- A long-running goroutine worker. It would make lifecycle and shutdown the
  lesson instead of SQS delivery semantics.
- Real AWS smoke tests. They would require cloud credentials, IAM state, and
  account cleanup.
- A DLQ implementation in the deterministic test path. DLQ/redrive policy is a
  production infrastructure concern; this example documents it while keeping the
  local contract focused on ack and retry visibility.

## Verification Shape

- Fake-client tests prove send body/attributes, receive policy, success delete,
  failure visibility change, invalid body retry, no-message handling, and
  cancellation before send.
- The opt-in Floci smoke test proves local SQS round trip, delete-after-success,
  and retry-visible failure without real AWS credentials.
- README diagrams separate static ownership from the success/failure operation
  sequence so the at-least-once and idempotency rules are visible.
