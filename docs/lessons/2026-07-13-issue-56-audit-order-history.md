# Issue #56 Audit Order History Lessons

## Context

The first 0.9.0-track workshop example needed to teach immutable order audit
history without absorbing the HTTP, SQL outbox, or Redis Streams lessons owned
by later issues. bluetape-go v0.18.0 already supplies validated audit events,
entries, in-memory storage, revision checks, duplicate detection, and queries.

## Decision

The service keeps a mutable teaching projection and appends one audit entry
before every state mutation. It holds a service mutex across the in-memory
append so revision selection, append, and projection assignment form one visible
critical section. Append success is the commit point: cancellation is checked
before and during append, never between successful append and the infallible map
assignment.

Stable caller-owned command IDs become both event IDs and idempotency keys.
Duplicate local retries return an `audit.ValidationError` compatible with both
`errors.Is(err, audit.ErrRevisionConflict)` and `errors.As`, matching
repository-originated conflicts.

## Surprise and Review Miss

The first command implementation validated input before checking whether the
service was configured. A zero-value service therefore returned
`ErrInvalidCommand` for malformed commands instead of the approved
`ErrInvalidConfig`. Pre-PR review found the ordering mismatch; a RED test fixed
the contract for zero values and nil receivers.

The first concurrency command matched only one of two intended tests and the
unrelated-order case asserted errors without checking every projection/history.
Both tests now share `^TestServiceConcurrent`, assert exact 16-goroutine
outcomes, and run 20 times before race detection.

The first `make ci` run failed at `revive` with 22 missing package/export doc
comments. The repository requires a comment for every exported error and status
constant, not only the surrounding block. Adding precise English API comments
reduced the result to `0 issues` before the full gate was rerun.

## Outcome and Proof

- Lifecycle, invalid transition, duplicate, repository failure, cancellation,
  defensive copy, bounded query, and concurrent reuse tests pass.
- The exact CLI JSON is protected by a golden file and a writer-error test.
- English and Korean READMEs state the same commands, behavior, and production
  limits.
- Focused tests, 20 stress repetitions, focused race tests, `go run`, diff
  checks, and repository-wide `make ci` provide the validation chain.

## Future Guard

Do not copy the process-wide lock into a durable adapter. Database-backed order
state and audit delivery need a caller-owned SQL transaction/outbox boundary,
storage pagination, retention, migration, access-control, and redaction policy.
Treat a query limit as a returned-cardinality bound only: the v0.18.0 memory
repository still scans and copies O(total stored entries).
