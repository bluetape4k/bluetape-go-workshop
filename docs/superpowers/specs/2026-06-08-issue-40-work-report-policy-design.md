# Issue #40 Design: Operations Report Policy Example

## Goal

Add a v0.4.0 workshop example that teaches how to build deterministic
application-facing output from `workreport.Report` and how
`workreport.FailurePolicy` changes aggregate behavior.

## Non-Goals

- Do not introduce a durable workflow engine, retry scheduler, queue, database,
  or observability dependency.
- Do not re-teach workflow runner composition from issue #39.
- Do not mutate `workreport.Report` or expose runtime timestamps as API contract.

## Example

- Path: `examples/operations-report-policy`
- Package: `internal/operations`
- HTTP framework: Gin
- Default port: `:8085`

## API

### `GET /healthz`

Returns `200 OK` with `{"status":"ok"}`.

### `POST /operations/report`

Request:

```json
{
  "run_id": "release-2026-06-08",
  "policy": "continue_on_failure",
  "products_valid": false,
  "retry_partner_notification": true,
  "skip_search_index": true
}
```

Rules:

- `run_id` is required after trimming whitespace.
- `policy` accepts `stop_on_failure` and `continue_on_failure`.
- `products_valid=false` makes `validate-products` fail.
- `retry_partner_notification=true` makes `notify-partner` preserve a failed
  first attempt and a completed retry as a nested partial report.
- `skip_search_index=true` records `refresh-search-index` as `aborted` with a
  caller-skip reason.
- A pre-cancelled request context returns a cancelled root report.

Response:

```json
{
  "run_id": "release-2026-06-08",
  "policy": "continue_on_failure",
  "status": "partial",
  "summary": {
    "completed": 3,
    "failed": 2,
    "partial": 2,
    "aborted": 1,
    "cancelled": 0,
    "total": 8,
    "terminal": true,
    "success": false,
    "failure": true
  },
  "report": {
    "name": "operations-run",
    "status": "partial",
    "children": []
  }
}
```

HTTP status mapping:

- `completed` -> `200 OK`
- `partial` -> `207 Multi-Status`
- `failed` or `aborted` -> `409 Conflict`
- `cancelled` -> `408 Request Timeout`
- invalid JSON/request/policy -> `400 Bad Request`

## Report Projection

The response uses a stable DTO:

- `name`
- `status`
- `error`
- `reason`
- `success`
- `failure`
- `partial`
- `cancelled`
- `children`

The DTO omits `StartedAt` and `EndedAt` because they are runtime facts and make
example assertions noisy.

## Diagrams

Generate and commit PNG plus SVG assets under `docs/images/readme-diagrams/`:

- `operations-report-policy-scenario`
- `operations-report-policy-architecture`
- `operations-report-policy-sequence`

READMEs embed PNG only and keep labels in English for generated diagrams.

## Tests

Focused tests must cover:

- health endpoint
- completed run
- continue-on-failure preserves validation failure, retry evidence, skip report,
  and deterministic summary counts
- stop-on-failure truncates children after the first failed child
- retry report structure
- invalid policy and invalid request bodies
- pre-cancelled request context mapping to cancelled report
- race test over the example package

## Production Hardening Notes

README hardening gaps must call out:

- persist reports if callers need audit history
- assign operation IDs/correlation IDs beyond the demo `run_id`
- connect report DTOs to logs/metrics/traces at the service boundary
- use a real retry policy/scheduler when retries cross request boundaries
- distinguish caller-skipped work from operator-aborted work if the domain needs
  separate status categories
