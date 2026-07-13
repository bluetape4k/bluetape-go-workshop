# Issue #58 Gin Audit Query API Lessons

## Context and Decision

The 0.9.0 workshop track needed an HTTP-shaped continuation of the order audit
history lesson. The example therefore exposes the released v0.18.0 audit query
model through Gin without moving query policy into a new workshop library.

Search uses POST `/audit/history/search` with a JSON body. This avoids practical
URL-length constraints and leaves room for structured revision and time filters.
Metadata is returned with each audit entry, but metadata filtering is deliberately
absent because `audit.Query` does not provide that contract in v0.18.0.

## Pagination Contract

The service asks the reader for `limit + 1` entries. The extra unseen entry proves
that another page exists and supplies its exclusive revision boundary. Ascending
queries return `next.from_revision`; descending queries return `next.to_revision`.
The next request includes that value, so no delivered revision is duplicated and
no opaque cursor format is invented.

The maximum page size is 100. This bounds returned and encoded data, not the total
work of the in-memory repository: its current implementation can still scan and
copy the stored history. Durable adapters need their own indexed pagination and
retention design.

## HTTP and Lifecycle Boundaries

The JSON boundary accepts only `application/json` with optional UTF-8 charset and
identity encoding. It rejects oversized, non-UTF-8, duplicate-key, unknown-field,
and trailing-value bodies. Aggregate identifiers are restricted to a small ASCII
path-safe alphabet, Gin keeps raw paths without unescaping, and forwarded proxy
addresses are not trusted.

The server binds to loopback by default. Remote unauthenticated exposure requires
an explicit environment opt-in and remains a demonstration-only mode. Read,
write, header, idle, request, and shutdown timeouts are bounded. Logs contain
route, method, status, public code, elapsed time, and lifecycle events rather than
request bodies, identifiers, metadata, or internal errors.

## Review Misses and Repairs

- The first `make ci` run exposed 18 missing package/export comments and three
  context-less HTTP test requests. The first focused lint rerun revealed two more
  instances after the initial report limit. All request constructors now receive
  an explicit context, all public symbols have useful comments, and focused lint
  reports `0 issues`.
- An overlength identifier fixture originally used a NUL-filled byte slice. It was
  replaced with a readable 129-character ASCII value without changing coverage.
- The first long-running full gate lost its process exit status even though its
  output was green. It was not accepted as proof; `make ci` was rerun from scratch
  with a captured `exit 0` marker.
- Interrupting `go run` proves signal handling but can make the wrapper exit
  ambiguously. A compiled binary was therefore exercised through health, POST
  search, detail lookup, SIGTERM, and `shutdown_completed`, ending with exit 0.

## Outcome and Future Guard

The example provides deterministic order history through a strict POST query and
exact revision endpoint, paired English/Korean guidance, stable public errors,
and normal/race/live verification. Reopen the design before adding authentication,
authorization, tenant isolation, metadata filtering, durable storage, opaque
cursors, streaming, or a public compatibility surface. Those changes alter the
trust, pagination, or ownership contract rather than merely extending the lesson.
