# Issue #56 Audit Order History Design

## Status

User-approved design for Issue #56, implemented against bluetape-go v0.18.0.
The Type A specification review converged at P0=0/P1=0 on 2026-07-13.

## Goal

Add a runnable, application-shaped order service that contrasts a mutable
current-state view with immutable audit history. The example demonstrates order
creation and status transitions, append-before-state consistency, duplicate
command detection, aggregate history, filtered queries, and deterministic JSON.

## Context and Current Evidence

Issue #56 is the first dependency in milestone track #35, before the Gin query
API (#58), SQL outbox publisher (#57), and Redis Streams integration (#68).
The current stable dependency is bluetape-go v0.18.0.

The released `audit` package provides `NewAggregateID`, `NewDomainEvent`,
`NewEntry`, `Repository.Append`, `LoadHistory`, `Find`, and the goroutine-safe
`MemoryRepository`. The repository validates contiguous revisions and rejects
globally duplicated event IDs and idempotency keys. It returns defensive copies
and preserves context cancellation. It is intentionally non-durable.

The library-local `examples/audit` already demonstrates create, add-item,
complete, and outbox replay. This workshop adopts its append-before-mutation
boundary but does not copy its scenario. The workshop focuses on status history,
current-state contrast, and query windows. Outbox replay is rejected here because
issues #57 and #68 own that lesson.

## Chosen Approach

Use a small `orderhistory.Service` with an injected `audit.Repository`, UTC
clock, mutex, and in-memory current-state map. Explicit `Create`, `Confirm`,
`Ship`, and `Cancel` methods make the state machine and its failure modes easy to
read. Every command validates and builds one fixed-size domain event, appends it,
and only then updates current state while holding the same service lock.

The runnable preview executes a fixed order lifecycle and prints deterministic
indented JSON containing the current snapshot, full aggregate history, and a
filtered history window. Repository values remain the audit source of truth;
the mutable map is only the teaching projection.

## Alternatives

### Direct repository fixture

A `main` function could append hand-built entries and print queries. This is
smaller, but it cannot teach where validation, revisions, idempotency, and state
mutation belong in an application boundary. Rejected as too thin.

### Durable SQL current state and outbox

A SQL transaction could update the order row and enqueue an audit message.
This is production-shaped, but it conflates the first audit lesson with
persistence and delivery. Deferred to issues #57 and #68.

### Event-sourced aggregate

Current state could be reconstructed exclusively by replaying audit events.
Rejected because the package explicitly models audit history, not an event
store. The README must state that history replay is not the recovery model.

## Package and Files

```text
examples/audit-order-history/
  main.go
  main_test.go
  README.md
  README.ko.md
  internal/orderhistory/
    model.go
    service.go
    service_test.go
    preview.go
    preview_test.go
```

Root `README.md` and `README.ko.md` will link the example. No dependency,
module, workflow, container, database, public library API, or diagram changes
are required.

## Service API Contract

The internal package exposes this compact teaching API:

```go
type Options struct {
    Author string
    Now    func() time.Time
}

type CreateCommand struct { OrderID, CommandID string }
type TransitionCommand struct { OrderID, CommandID string }
type CancelCommand struct { OrderID, CommandID, Reason string }

func NewService(repo audit.Repository, options Options) (*Service, error)
func (s *Service) Create(context.Context, CreateCommand) (Order, error)
func (s *Service) Confirm(context.Context, TransitionCommand) (Order, error)
func (s *Service) Ship(context.Context, TransitionCommand) (Order, error)
func (s *Service) Cancel(context.Context, CancelCommand) (Order, error)
func (s *Service) Current(orderID string) (Order, bool)
func (s *Service) History(context.Context, string) (audit.History, bool, error)
func (s *Service) Find(context.Context, audit.Query) ([]audit.Entry, error)
```

`NewService` rejects a nil repository and invalid author with
`ErrInvalidConfig`; a nil clock defaults to `time.Now().UTC`. Nil contexts are
normalized to `context.Background`, matching the audit package. `Service{}` is
not a usable configuration: every method detects missing dependencies and
returns `ErrInvalidConfig` (or `false` for `Current`) instead of panicking.

Stable service sentinels are `ErrInvalidConfig`, `ErrInvalidCommand`,
`ErrOrderExists`, `ErrOrderNotFound`, and `ErrInvalidTransition`. Construction,
ID, reason, and state-machine errors wrap the matching sentinel for
`errors.Is`. A `Find` limit above 100 or incompatible aggregate type wraps
`audit.ErrInvalidQuery`. Repository errors are never translated away.

## Domain Model and State Machine

`Order` contains a canonical order ID, status, revision, and UTC update time.
The supported statuses are `pending`, `confirmed`, `shipped`, and `cancelled`.

Allowed transitions are:

| Command | From | To |
|---|---|---|
| Create | absent | pending |
| Confirm | pending | confirmed |
| Ship | confirmed | shipped |
| Cancel | pending or confirmed | cancelled |

`shipped` and `cancelled` are terminal. Creating an existing order, acting on a
missing order, or requesting any other transition returns a stable sentinel
error without writing audit history or current state.

Order IDs and command IDs are trimmed and must match
`[A-Za-z0-9][A-Za-z0-9._-]{0,127}`. This prevents ambiguous canonicalization and
keeps repository keys bounded. The caller supplies a unique command ID. That
same canonical value is used as both `EventID` and `IdempotencyKey`; it is never
derived by lossy concatenation. This is duplicate detection, not successful
idempotent replay: resubmitting a committed command fails and preserves
`errors.Is(err, audit.ErrRevisionConflict)`.

Each event uses aggregate type `order` and an `audit.AggregateID` for the order.
Event types are `order.created`, `order.confirmed`, `order.shipped`, and
`order.cancelled`. Revision is the prior current-state revision plus one.
Changes are fixed application-owned fields: `status` before/after and an
optional cancellation reason that must be valid UTF-8 and no longer than 256
runes. No arbitrary caller JSON is accepted. The author is fixed by service
configuration, trimmed, required, and limited to 128 runes.

## Consistency and Concurrency

Each command checks context, validates input, acquires the service mutex,
rechecks context, validates current state, constructs the event and entry, and
calls `Repository.Append` while still holding the mutex. Only a successful
append mutates the map. This serializes revision selection and prevents two
concurrent commands for one order from racing. The example intentionally favors
a simple, visible invariant over per-aggregate lock machinery.

Append success is the commit point. Cancellation before or during append causes
the repository to return an error and neither side changes. Once append succeeds,
the in-memory state update is infallible and completes under the lock even if
cancellation arrives immediately afterward. Checking cancellation between those
two operations would create an audit/current-state split and is forbidden.

Repository errors are wrapped with operation context while preserving
`errors.Is`. Duplicate event and idempotency-key conflicts both preserve
`audit.ErrRevisionConflict`; callers may use `errors.As` to inspect the
`audit.ValidationError.Field` when they need to distinguish them. Returned
orders, preview slices, and repository history are copied; callers cannot mutate
stored state.

Holding the lock across a repository call is acceptable only because this
example uses the bounded in-memory repository and has no network or database
I/O. It is a demo-scale service that intentionally serializes unrelated orders
and makes no throughput or horizontal-scaling claim. The README states that a
durable application must use a caller-owned SQL transaction/outbox boundary
rather than hold a process mutex across I/O, and should use per-aggregate or
database-owned concurrency instead of this service-wide lock.

## Query and Preview Contract

`Service.History` delegates to `LoadHistory` for one aggregate. It is an
aggregate-demo-only full read, not a production pagination contract.
`Service.Find` accepts an `audit.Query`, preserves inclusive revision/time
filters and newest-first ordering, forces or validates aggregate type `order`,
replaces an omitted limit with 20, and rejects limits above 100. This prevents a
shared repository from turning the workshop service into a cross-domain query
surface. Both methods check context. An absent aggregate returns
`(audit.History{}, false, nil)` from `History`; `Find` returns a non-nil empty
`[]audit.Entry` when no entries match. The README warns that production history
APIs need storage-backed pagination and retention rather than unbounded full
copies.

`BuildPreview` creates one order and runs create, confirm, and ship with fixed
command IDs and injected timestamps. It returns:

```json
{
  "current": {"order_id":"order-1001","status":"shipped","revision":3},
  "history": [],
  "recent_history": []
}
```

The actual history arrays contain stable event identity, type, revision,
occurred-at time, author, and status changes. `recent_history` demonstrates a
revision range and newest-first limit. `main.go` uses a deterministic UTC clock,
marshals with indentation, and writes only the JSON document to stdout.

## Failure Modes

1. Invalid or duplicate commands fail before current-state mutation. If the
   repository detects a reused global event/idempotency identity, the command
   returns the repository error and leaves the projection unchanged.
2. A repository failure or cancellation during append leaves both history and
   current state unchanged.
3. Concurrent commands are serialized. Exactly one valid transition may win;
   later commands observe the resulting state and either continue validly or
   fail with an invalid transition. No revision gap or race is allowed.
4. Mutation of a returned order or preview value cannot alter stored state or
   later query results.

## Security and Operations Boundaries

The example has no HTTP server, authentication, secret, external process, or
durable store. Cancellation reasons are bounded to 256 UTF-8 runes and are
demonstration metadata, not a place for PII or credentials. Event payloads and
logs must be classified and redacted by the real application.

`MemoryRepository` loses all data on process exit. Production owners must
choose retention, deletion, archival, access control, schema versioning and
migration, payload-size limits, encryption, and PII/redaction policy. This
example provides neither disaster recovery nor exactly-once delivery.

## Testing

Focused tests will prove:

- create-confirm-ship ordering, revisions, event metadata, history, and queries;
- cancel from pending and confirmed, plus terminal/invalid transitions;
- missing/duplicate orders and duplicate command/event identities;
- repository failure and context cancellation with no projection mutation;
- successful append as commit point despite cancellation immediately afterward;
- invalid IDs and bounded cancellation reason;
- defensive copies, absent-history `false`, and non-nil empty query results;
- deterministic preview JSON; and
- bounded concurrent reuse under `go test -race` without sleeps.

Validation order is focused package tests, focused race tests, runnable preview,
`git diff --check`, and repository-wide `make ci`.

## Compatibility, Migration, and Rollback

The example adds only new workshop files and uses the existing v0.18.0 module
dependency. It does not change public APIs. Rollback is deletion of the example,
its root navigation links, and its documentation artifacts. A future durable
implementation must not treat `MemoryRepository` data as migratable production
state; it should introduce an explicit persistence and outbox design in the
follow-up issues.

## Acceptance Criteria and DoD

- The application-shaped service implements all defined transitions and the
  append-before-mutation consistency contract.
- The preview deterministically contrasts current state, full immutable history,
  and a filtered query.
- Focused success, failure, edge, cancellation, defensive-copy, and concurrent
  reuse tests pass, including race detection.
- English and Korean README files explain the lesson, run command, expected
  behavior, audit-versus-event-sourcing boundary, and operational gaps.
- Root English and Korean navigation links the runnable example.
- Type A spec, plan, verifier, review, lessons, PR, CI, merge, sync, and cleanup
  gates complete with P0=0/P1=0.

## Specification Review Record

| Lens | Result | Resolution |
|---|---|---|
| Performance | P0=0, P1=0, P2=2 | Documented demo-scale global serialization and bounded `Find`; full history is explicitly demo-only. |
| Stability | P0=0, P1=0 | Commit-point cancellation, failure atomicity, and bounded race proof are explicit. |
| Security | P0=0, P1=0 | IDs, author, reason, query domain, metadata, UTF-8, and PII boundaries are bounded. |
| Operator/Ops | P0=0, P1=0 | Durability, retention, migration, rollback, recovery, and diagnostics boundaries are explicit. |
| Developer/API | P0=0, P1=0 after repair | Added exact API, zero-value/config/error semantics, duplicate retry behavior, and absent-history contract. |
| User/caller | P0=0, P1=0 | Preview, README lesson, unsupported behavior, and production misuse warnings are explicit. |
| Main integration | P0=0, P1=0 | No unresolved contradiction, scope expansion, dependency, or repository hazard remains. |

The stability, security, operator, and user lanes timed out after bounded waits;
the required fallback reviews were completed independently in the main session.
