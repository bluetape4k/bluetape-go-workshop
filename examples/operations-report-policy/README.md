# Operations Report Policy

[English](README.md) | [한국어](README.ko.md)

This example exposes a small Gin HTTP API that turns `workreport.Report` values
into deterministic response JSON. It focuses on
`github.com/bluetape4k/bluetape-go/workreport` aggregation and shows how
`StopOnFailure` and `ContinueOnFailure` change the shape of an operations run.

## Example Scenario

The API receives one operations checklist run. The run loads a catalog snapshot,
validates product data, optionally records a partner notification retry, refreshes
the search index, and writes a final run summary. The caller chooses whether the
run stops at the first non-completed report or continues and preserves every
child report.

![Operations report policy scenario](../../docs/images/readme-diagrams/operations-report-policy-scenario.png)

## Report Fields

| Field | Purpose |
|---|---|
| `name` | Stable operation name used in logs, dashboards, and tests. |
| `status` | One of `completed`, `failed`, `partial`, `aborted`, or `cancelled`. |
| `error` | Error message for failed or cancelled reports. |
| `reason` | Caller-defined reason for an aborted report. This example uses it for skipped work. |
| `success` | True only for `completed`. |
| `failure` | True for `failed`, `partial`, `aborted`, or `cancelled`. |
| `partial` | True for aggregate reports that preserved one or more non-completed children. |
| `children` | Nested operation outcomes. |

The response deliberately omits `StartedAt` and `EndedAt` from
`workreport.Report`. Those timestamps are runtime facts, so hiding them keeps the
API contract and tests deterministic.

## Failure Policies

| Policy | Behavior |
|---|---|
| `stop_on_failure` | Stops executing the checklist after the first non-completed report and returns only reports that actually ran. |
| `continue_on_failure` | Runs the remaining checklist steps and returns a `partial` root report when any child is non-completed. |

Retry evidence is modeled as a nested `notify-partner` report. A failed first
attempt plus a completed retry remains `partial`; the example does not pretend
that preserved failed attempts are full success. Caller-skipped search index work
is represented as `aborted` with reason `skipped by request` because
`workreport` has no dedicated `skipped` status.

## Run

```bash
go run ./examples/operations-report-policy
```

Optional environment variables:

| Variable | Default | Purpose |
|---|---:|---|
| `HTTP_ADDR` | `:8085` | Listening address. |

## Endpoints

```bash
curl http://localhost:8085/healthz
curl -X POST http://localhost:8085/operations/report \
  -H 'Content-Type: application/json' \
  -d '{
    "run_id": "release-1001",
    "policy": "continue_on_failure",
    "products_valid": true
  }'
```

Useful variants:

```bash
# Preserve validation failure, retry evidence, skipped work, and final summary.
curl -X POST http://localhost:8085/operations/report \
  -H 'Content-Type: application/json' \
  -d '{
    "run_id": "release-partial",
    "policy": "continue_on_failure",
    "products_valid": false,
    "retry_partner_notification": true,
    "skip_search_index": true
  }'

# Fail fast after validate-products.
curl -X POST http://localhost:8085/operations/report \
  -H 'Content-Type: application/json' \
  -d '{
    "run_id": "release-stop",
    "policy": "stop_on_failure",
    "products_valid": false,
    "retry_partner_notification": true,
    "skip_search_index": true
  }'
```

Completed reports return `200 OK`. Partial reports return `207 Multi-Status`.
Failed or aborted reports return `409 Conflict`. Cancelled reports return
`408 Request Timeout`. Invalid JSON, blank `run_id`, missing `products_valid`,
and unknown policy values return `400 Bad Request`.

## When This Is Enough

This pattern is enough when one HTTP request can own the run, the service only
needs to publish a deterministic report tree, and partial outcomes can be
interpreted by the caller.

Use a durable job system instead when reports must survive service restarts,
retries cross request boundaries, or operators need to resume work later. Add
real operation IDs, correlation IDs, logs, metrics, traces, and persistent audit
storage before using the pattern for production operations.

## Architecture

Gin owns JSON binding and HTTP status mapping. The operations layer owns the
checklist and policy selection. `workreport.Aggregate` owns the parent status and
child preservation semantics. The response mapper projects reports into stable
JSON and builds summary counts over the root report plus all descendants.

![Operations report policy architecture](../../docs/images/readme-diagrams/operations-report-policy-architecture.png)

## Sequence Diagram

The handler validates input, chooses a failure policy, executes checklist steps,
aggregates the child reports, and returns a stable report projection.

![Operations report policy sequence](../../docs/images/readme-diagrams/operations-report-policy-sequence.png)

## Test

```bash
go test -count=1 ./examples/operations-report-policy/...
go test -race -count=1 ./examples/operations-report-policy/...
```
