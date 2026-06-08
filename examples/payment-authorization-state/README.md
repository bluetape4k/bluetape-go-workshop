# Payment Authorization State

[English](README.md) | [한국어](README.ko.md)

This example exposes one in-memory payment authorization through a small Gin HTTP
API. It builds on the broader
[`order-lifecycle-state-api`](../order-lifecycle-state-api) example and narrows
the lesson to payment-specific state transitions plus app-layer idempotent retry
behavior.

## Example Scenario

A payment starts in `requested`. The API can authorize it when the amount is
positive, capture an authorized payment, fail a requested or authorized payment,
or cancel a requested or authorized payment. `captured`, `failed`, and
`cancelled` are final states. Transition commands require an `idempotency_key`
so callers can safely retry a successful command without mutating the state
again.

![Payment authorization state scenario](../../docs/images/readme-diagrams/payment-authorization-state-scenario.png)

## State Model

| From | Event | To | Rule |
|---|---|---|---|
| `requested` | `authorize` | `authorized` | `amount_cents` must be positive. |
| `authorized` | `capture` | `captured` | `captured` is final. |
| `requested`, `authorized` | `fail` | `failed` | `failed` is final. |
| `requested`, `authorized` | `cancel` | `cancelled` | `cancelled` is final. |

`allowed_events` reports registered events for the current state. It does not
evaluate guards. Use `/payments/current/transitions/:event/can` when a caller
needs to evaluate a guard such as the positive-amount authorization rule.

## Idempotent Retry

The `state` package owns transition legality. Idempotency is application-layer
behavior around the state machine:

| Retry case | Response |
|---|---|
| Same `idempotency_key`, same `event` after a successful transition | Returns the stored transition response with `idempotent_replay=true`. |
| Same `idempotency_key`, different `event` | Returns `409 Conflict` with code `idempotency_conflict`. |
| Failed transition followed by a valid transition with the same key | Runs normally because failed transitions are not stored as successful replay responses. |

This example keeps idempotency in memory. Production payment APIs need durable
idempotency storage, TTL, request-hash validation, and audit logging.

## Run

```bash
go run ./examples/payment-authorization-state
```

Optional environment variables:

| Variable | Default | Purpose |
|---|---:|---|
| `HTTP_ADDR` | `:8086` | Listening address. |
| `PAYMENT_ID` | `pay-1001` | Example payment identifier. |
| `PAYMENT_AMOUNT_CENTS` | `12900` | Amount used by the positive-amount guard. |

## Endpoints

```bash
curl http://localhost:8086/healthz
curl http://localhost:8086/payments/current
curl http://localhost:8086/payments/current/transitions/authorize/can
curl -X POST http://localhost:8086/payments/current/transitions \
  -H 'Content-Type: application/json' \
  -d '{"event":"authorize","idempotency_key":"auth-1"}'
```

Useful variants:

```bash
# Idempotent replay of the same command.
curl -X POST http://localhost:8086/payments/current/transitions \
  -H 'Content-Type: application/json' \
  -d '{"event":"authorize","idempotency_key":"auth-1"}'

# Capture after authorization.
curl -X POST http://localhost:8086/payments/current/transitions \
  -H 'Content-Type: application/json' \
  -d '{"event":"capture","idempotency_key":"capture-1"}'
```

Transition commands return `409 Conflict` when the event is not valid from the
current state, a final state rejects further transitions, a guard rejects the
event, a concurrent request loses the state-change race, or an idempotency key is
reused for a different event. Malformed JSON, missing fields, and unknown events
return `400 Bad Request`.

## When This Is Enough

A finite state machine plus app-layer idempotency is enough when the service only
needs to protect one short-lived payment authorization from impossible commands
and safe duplicate retries.

Use a workflow runner or durable job system when authorization, capture, refund,
or reversal spans multiple services, needs compensation, waits on humans, or
must survive restarts. Use durable storage for both payment state and
idempotency keys before adapting this pattern to production.

## Architecture

Gin owns HTTP routing and JSON binding. The payment handler parses commands,
serializes transition plus idempotency writes, and maps package sentinel errors
to stable HTTP responses. `state.Machine` owns transition legality, guard
evaluation, final-state rejection, and concurrent transition conflicts.

![Payment authorization state architecture](../../docs/images/readme-diagrams/payment-authorization-state-architecture.png)

## Sequence Diagram

The main request path checks for an idempotent replay before calling
`state.Machine.Transition`. Successful transitions are stored by idempotency key;
failed transitions are not stored as replayable success responses.

![Payment authorization state sequence](../../docs/images/readme-diagrams/payment-authorization-state-sequence.png)

## Test

```bash
go test -count=1 ./examples/payment-authorization-state/...
go test -race -count=1 ./examples/payment-authorization-state/...
```
