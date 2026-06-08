# Issue #70 Design: Payment Authorization State Example

## Goal

Add a focused v0.4.0 example that demonstrates payment authorization state
transitions, invalid transition errors, and idempotent retry behavior using
`state.Machine`.

## Non-Goals

- Do not call a real payment gateway.
- Do not add a database, queue, durable idempotency store, or workflow runner.
- Do not hide `state.Machine` behind a large application framework.
- Do not duplicate the broader order lifecycle example from #38.

## Example

- Path: `examples/payment-authorization-state`
- Package: `internal/paymentauth`
- HTTP framework: Gin
- Default port: `:8086`

## State Model

| From | Event | To | Rule |
|---|---|---|---|
| `requested` | `authorize` | `authorized` | `amount_cents` must be positive. |
| `authorized` | `capture` | `captured` | Captured is final. |
| `requested`, `authorized` | `fail` | `failed` | Failed is final. |
| `requested`, `authorized` | `cancel` | `cancelled` | Cancelled is final. |

Final states reject further transition attempts.

## API

### `GET /healthz`

Returns `200 OK` with `{"status":"ok"}`.

### `GET /payments/current`

Returns:

```json
{
  "payment_id": "pay-1001",
  "state": "requested",
  "amount_cents": 12900,
  "allowed_events": ["authorize", "fail", "cancel"]
}
```

### `GET /payments/current/transitions/:event/can`

Evaluates whether an event can run from the current state. Guard rejection maps
to `409 Conflict`; unavailable events return `200 OK` with `allowed=false`.

### `POST /payments/current/transitions`

Request:

```json
{
  "event": "authorize",
  "idempotency_key": "auth-1"
}
```

Response:

```json
{
  "payment": {},
  "previous": "requested",
  "event": "authorize",
  "current": "authorized",
  "idempotent_replay": false
}
```

Rules:

- `event` and `idempotency_key` are required after trimming whitespace.
- Repeating the same `idempotency_key` with the same event returns the stored
  successful transition response with `idempotent_replay=true`.
- Reusing the same `idempotency_key` for a different event returns
  `409 Conflict` with code `idempotency_conflict`.
- Unknown events return `400 Bad Request`.
- Invalid transitions, final-state transitions, guard rejections, and
  concurrent transition conflicts return `409 Conflict`.
- Cancelled/deadline contexts return `408 Request Timeout`.

## Implementation Notes

- `state.Machine` remains the source of truth for current state and allowed
  events.
- A small `idempotencyStore` protects successful transition responses with a
  mutex.
- Transition plus idempotency record writes are serialized by a server-level
  mutex so replay metadata and state changes remain coherent.
- Snapshot responses use stable DTOs.

## Diagrams

Generate and commit PNG plus SVG assets under `docs/images/readme-diagrams/`:

- `payment-authorization-state-scenario`
- `payment-authorization-state-architecture`
- `payment-authorization-state-sequence`

READMEs embed PNG only and keep generated labels in English.

## Tests

Focused tests must cover:

- health/current endpoint
- allowed authorize/capture path
- fail and cancel paths to final states
- capture-before-authorize invalid transition
- guard rejection for non-positive amount
- same-key idempotent replay without additional mutation
- same-key different-event conflict
- final-state rejection
- malformed/missing/unknown event requests
- concurrent duplicate authorize safety
- race test over the example package

## Production Hardening Notes

README hardening gaps must call out:

- durable payment state storage
- durable idempotency-key storage with TTL and request hash validation
- real payment gateway integration
- audit logging and observability
- separation between authorization failure and operator cancellation in domain
  reporting
