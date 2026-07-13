# Issue #58 Gin Audit Query API Design

## Status

User-approved design for Issue #58, ready for implementation against
bluetape-go v0.18.0.
The approved delivery boundary includes PR creation, green CI, rebase merge,
local `develop` synchronization, and cleanup of the owned worktree.

## Goal

Add a runnable Gin service that queries immutable order audit entries through a
strict, bounded HTTP API while keeping storage and pagination rules independent
of Gin. The example teaches aggregate-scoped history search, revision-based
continuation, detail lookup, typed audit errors, and the trust boundary around
raw audit payloads and metadata.

## Context and Current Evidence

Issue #58 follows the completed audit order history example (#56) in milestone
track #35 and precedes the SQL outbox examples (#57 and #68). The current stable
dependency and workshop module baseline is bluetape-go v0.18.0.

The released `audit.HistoryReader` exposes `Find`, `LoadHistory`, `Latest`, and
snapshot reads without exposing persistence. `audit.Query` supports an exact
aggregate, aggregate type, inclusive revision and recorded-at ranges,
newest-first ordering, and a limit. `audit.MemoryRepository` is goroutine-safe,
checks context, returns defensive copies, and applies the limit after filtering
and ordering. It does not offer a metadata predicate or an event-ID lookup.

The existing `gin-content-moderation-workflow` provides the repository's current
Gin boundary pattern: strict bounded JSON, disabled proxy trust, stable public
errors, loopback-safe defaults, owned server timeouts, and joined graceful
shutdown. This example borrows those boundary rules but owns a smaller audit
query contract.

## Chosen Approach

Use an internal `auditquery.Service` over an injected `audit.HistoryReader`.
Search requires one canonical aggregate and accepts filters in a POST JSON body.
The service converts the request to `audit.Query`, requests `limit + 1`, returns
at most the requested limit, and exposes the first unseen revision as the next
inclusive boundary. Detail lookup uses the same `Find` API with an exact
aggregate and revision.

The runnable application seeds a `MemoryRepository` with deterministic order
events, builds a Gin engine, and serves only read operations. Gin owns transport
validation, request deadlines, redacted public errors, safe diagnostics, and
body closure. `main` owns listener policy and shutdown.

## Alternatives

### GET search with query parameters

This is familiar for simple filters, but revision/time filters and future
extensions make the URL long and awkward. It also conflicts with the approved
decision to submit structured search input as JSON. Rejected; only the compact
detail identity remains a GET path.

### Global cross-aggregate search

`audit.Query` can query by aggregate type or all entries, but revisions are only
ordered within an aggregate. A revision continuation token is therefore
ambiguous across aggregates and a global query expands authorization and tenant
isolation risk. Rejected for this workshop API.

### Application-owned metadata filtering

The service could fetch entries and filter `Event.Metadata` itself. Because the
repository applies `Limit` first, that would produce incomplete or incorrect
pages unless the application scanned an unbounded result. Rejected until the
library or durable store supplies a metadata-aware query contract.

## Package and Files

```text
examples/gin-audit-query-api/
  main.go
  main_test.go
  README.md
  README.ko.md
  internal/auditquery/
    model.go
    service.go
    service_test.go
    fixture.go
    fixture_test.go
    server.go
    server_test.go
```

Root `README.md` and `README.ko.md` will link the example. The Type A spec,
plan, review, and lesson artifacts remain under the repository's existing docs
paths. No module, dependency, workflow, container, database, changelog, public
bluetape-go API, or diagram changes are required.

## Service API Contract

The framework-independent package exposes this teaching surface:

```go
type ServiceConfig struct {
    DefaultLimit int
    MaximumLimit int
}

type AggregateRequest struct {
    Type string `json:"type"`
    ID   string `json:"id"`
}

type SearchRequest struct {
    Aggregate      AggregateRequest `json:"aggregate"`
    FromRevision   audit.Revision   `json:"from_revision,omitempty"`
    ToRevision     audit.Revision   `json:"to_revision,omitempty"`
    FromRecordedAt time.Time        `json:"from_recorded_at,omitempty"`
    ToRecordedAt   time.Time        `json:"to_recorded_at,omitempty"`
    NewestFirst    bool             `json:"newest_first,omitempty"`
    Limit          int              `json:"limit,omitempty"`
}

type NextPage struct {
    FromRevision audit.Revision `json:"from_revision,omitempty"`
    ToRevision   audit.Revision `json:"to_revision,omitempty"`
}

type Page struct {
    Limit   int       `json:"limit"`
    HasMore bool      `json:"has_more"`
    Next    *NextPage `json:"next,omitempty"`
}

type SearchResponse struct {
    Entries []audit.Entry `json:"entries"`
    Page    Page          `json:"page"`
}

func NewService(audit.HistoryReader, ServiceConfig) (*Service, error)
func (s *Service) Search(context.Context, SearchRequest) (SearchResponse, error)
func (s *Service) Get(context.Context, AggregateRequest, audit.Revision) (audit.Entry, error)
```

`NewService` rejects nil and typed-nil readers, non-positive defaults or
maximums, a default above the maximum, and a maximum above 100. A zero-value
`Service` fails closed with `ErrInvalidConfig`. Nil contexts normalize to
`context.Background`, matching the audit package.

Stable sentinels are `ErrInvalidConfig`, `ErrInvalidRequest`, and
`ErrEntryNotFound`. Service validation wraps its sentinel and preserves
underlying `audit.ErrInvalidAggregateID`, `audit.ErrInvalidRevision`, or
`audit.ErrInvalidQuery` when present. Reader failures are wrapped with `%w` and
remain inspectable with `errors.Is` and `errors.As`.

Aggregate type and ID are trimmed and must match
`[A-Za-z0-9][A-Za-z0-9._-]{0,127}` before `audit.NewAggregateID` is called.
This path-safe policy avoids decoding ambiguity and bounds caller-controlled
keys. Revision zero is valid only as an omitted search bound; detail lookup
requires a valid nonzero `audit.Revision`.

## Pagination and Filtering Contract

Search always supplies the exact aggregate to `audit.Query`. Revision and
recorded-at ranges are inclusive, and the repository's append order is used
unless `newest_first` is true. The service uses the configured default when
`limit` is zero and rejects negative limits or limits above the maximum.

The repository receives `effectiveLimit + 1`. If no extra entry exists,
`has_more` is false and `next` is omitted. Otherwise the extra entry is not
returned and its revision becomes the next inclusive boundary:

- ascending search returns `next.from_revision`;
- newest-first search returns `next.to_revision`.

The caller resubmits the original JSON filters and replaces only the returned
revision boundary. Using the first unseen revision avoids duplicates and gaps,
including when recorded-at filters omit intermediate revisions. An empty match
is a successful response with a non-nil empty `entries` array. Search never
returns 404; detail lookup returns `ErrEntryNotFound` when the exact revision is
absent.

## HTTP Contract

Routes are:

```text
GET  /healthz
POST /audit/history/search
GET  /audit/aggregates/:type/:id/revisions/:revision
```

The POST route requires `application/json` with optional UTF-8 charset,
identity content encoding, valid UTF-8, a 32 KiB maximum body, one JSON object,
no duplicate keys, no unknown fields, and no trailing JSON value. Gin uses the
raw escaped path without automatic unescaping or trailing-slash redirects;
path values must satisfy the canonical identifier pattern.

Successful search responses use `SearchResponse`. Successful detail responses
use `{"entry": <audit.Entry>}`. Audit entries intentionally retain their JSON
payload, event metadata, author, change metadata, and snapshot metadata. Empty
slices are encoded as `[]`, not `null`.

Public errors use:

```json
{"error":{"code":"invalid_audit_query","message":"audit query is invalid"}}
```

Malformed transport input and service `ErrInvalidRequest` map to 400. Typed
audit aggregate/query/revision validation maps to 400 with
`invalid_audit_query`; this more specific mapping is evaluated before the own
sentinel because service errors may wrap both. Missing detail maps to 404.
Adapter-owned deadline expiry maps to 408. Unknown reader/service failures map
to 500. Raw error values, aggregate IDs, payloads, metadata, and request bodies
are never included in the public message or log.

The adapter applies a two-second service deadline, closes every request body,
disables trusted proxies, enables method-not-allowed handling, and logs only
route template, method, status, low-cardinality code, and elapsed time. Caller
cancellation is preserved internally and does not trigger an unreliable write
after the client disconnects.

## Fixture and Runtime Contract

`SeedRepository` appends deterministic, validated `audit.Entry` values for two
order aggregates. `order-1001` has created, confirmed, packed, and shipped
events; `order-2001` has created and cancelled events. IDs, idempotency keys,
authors, UTC timestamps, payloads, changes, and safe workshop metadata are
fixed. Seeding fails immediately if any constructor or append fails.

The default address is `127.0.0.1:8080`. A non-loopback `HTTP_ADDR` is rejected
unless `ALLOW_UNAUTHENTICATED_REMOTE=1`. The `http.Server` owns header, read,
write, idle, and maximum-header limits. Signal-driven shutdown has a five-second
bound, joins the listen goroutine, and forces `Close` if graceful shutdown
fails. No public listener is opened in tests.

## Failure Modes

1. Malformed, oversized, duplicate-key, unsupported-media, or unknown-field
   JSON is rejected before the service is called.
2. Invalid aggregate, revision/time range, direction boundary, or limit returns
   a stable 400 while preserving typed audit errors internally.
3. A missing detail revision returns 404; an empty search remains 200 with an
   empty array so callers can distinguish lookup absence from an empty page.
4. Reader cancellation, deadline, or failure is propagated without retry. The
   HTTP adapter distinguishes its own timeout from client cancellation and
   never logs raw audit content.
5. A future metadata predicate cannot be silently added above a limited reader;
   doing so would violate page completeness and requires a new approved query
   contract.
6. Failed graceful shutdown triggers forced close and joins the listener; no
   server goroutine may be left behind.

## Security and Operations Boundaries

The example intentionally has no authentication, authorization, tenant policy,
rate limiting, durable retention, encryption, or field-level redaction. It is
safe by default only because it binds to loopback. Production deployment must
authorize both aggregate identity and every returned payload/metadata field,
derive tenant scope from authenticated server context rather than request JSON,
apply retention/deletion policy, and rate-limit expensive history reads.

`/healthz` proves process availability only. `MemoryRepository` loses all data
on restart and is not readiness or durability proof. Diagnostics are
low-cardinality and intentionally omit caller-controlled identifiers. The
README must not describe the example as event sourcing, recovery, global audit
search, exactly-once delivery, or production-ready access control. The fixed
fixture and page limit bound this demo's responses; a production repository
must also enforce per-entry payload/metadata and total response byte budgets.

## Testing

Focused tests prove:

- constructor, typed-nil, zero-value, ID, revision/time, and limit validation;
- ascending and newest-first `limit + 1` pagination without duplicates or gaps;
- inclusive revision/time filters, empty pages, detail success/not-found, and
  preservation of `errors.Is`/`errors.As` reader failures;
- deterministic validated fixture entries;
- strict media type/encoding/UTF-8/body-size/duplicate/unknown/trailing input,
  method and path behavior, response shape, body closure, timeout, caller
  cancellation, redacted errors, and low-cardinality logs;
- loopback policy, server timeout defaults, graceful shutdown, forced close,
  listener failure, and goroutine join; and
- concurrent searches and detail reads under `go test -race` against the
  goroutine-safe memory repository with exact response assertions.

Validation order is focused RED/GREEN package tests, focused race tests, a
loopback `go run` plus curl smoke check, `git diff --check`, and repository-wide
`make ci`.

## Compatibility, Migration, and Rollback

The example adds only workshop files and uses existing dependencies. It changes
no public library API and requires no migration. Rollback is deletion of the
example, its root navigation entries, and its durable workflow artifacts. The
fixture is disposable and has no state migration path.

## Acceptance Criteria and DoD

- Search uses POST JSON and maps exactly to the supported v0.18.0 audit query
  surface without inventing metadata filtering.
- Framework-independent service tests prove filters, both pagination
  directions, detail lookup, empty results, typed errors, cancellation, and
  concurrent read safety.
- Gin and server tests prove strict bounded input, safe errors/logging,
  body/resource ownership, request/server timeouts, and joined shutdown.
- English and Korean README files include run/curl examples, expected response
  shapes, continuation instructions, and trust-boundary warnings; root
  navigation links the example in both locales.
- Spec, plan, implementation, review, lesson, PR, CI, rebase merge, local sync,
  and owned-worktree cleanup complete with P0=0 and P1=0.

## Specification Review Record

The required installed-role dispatch could not be performed because the active
subagent interface does not expose the mandatory `agent_type` field. The main
session therefore performed six isolated reads of this exact artifact, then
integrated the results as the workflow fallback.

| Lens | Result | Resolution |
|---|---|---|
| Performance | P0=0, P1=0, P2=1 | Kept pages at at most 100 plus one lookahead and documented production entry/response byte budgets. |
| Stability | P0=0, P1=0 | Context ownership, inclusive continuation, body closure, timeout behavior, forced close, and listener join are explicit. |
| Security | P0=0, P1=0 | Aggregate scope, strict input, proxy policy, loopback default, safe logs, authorization gaps, and remote opt-in are explicit. |
| Operator/Ops | P0=0, P1=0 | Health semantics, in-memory durability, diagnostics, shutdown, rollback, retention, and rate-limit boundaries are explicit. |
| Developer/API | P0=0, P1=0 after repair | Clarified specific audit-error mapping precedence, typed-nil/zero-value behavior, and exact first-unseen continuation contract. |
| User/caller | P0=0, P1=0 | POST JSON, empty-search versus detail-not-found, returned next boundary, unsupported metadata filter, and misuse warnings are explicit. |
| Main integration | P0=0, P1=0 | Corrected pre-implementation status; no unresolved contradiction, scope expansion, dependency, or repository hazard remains. |
