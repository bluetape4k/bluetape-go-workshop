# resilience-http-web

HTTP service example that composes `bluetape-go/resilience` retry, timeout,
circuit breaker, bulkhead, and event hooks.

## Run

Start a catalog service on `:9090`, then run this service:

```bash
export CATALOG_URL=http://localhost:9090
go run ./examples/resilience-http-web
```

## API

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Health check. |
| `GET` | `/catalog/{id}` | Calls the configured catalog service through retry, timeout, and circuit breaker policies. |
| `POST` | `/orders?delay=25ms` | Handles an order through a reject-mode bulkhead with one concurrent slot. |
| `GET` | `/events` | Returns low-cardinality resilience events emitted by the policies. |

## What It Demonstrates

- Retry + timeout + circuit breaker composition for outbound `net/http` calls.
- HTTP 5xx conversion into retryable `resilience.StatusError` values.
- Circuit-open typed error handling with `errors.Is(err, resilience.ErrCircuitOpen)`.
- Reject-mode bulkhead protection for inbound handlers.
- Bulkhead typed rejection handling with `errors.Is(err, resilience.ErrBulkheadRejected)`.
- Synchronous event hooks without adding a logging or metrics dependency.
