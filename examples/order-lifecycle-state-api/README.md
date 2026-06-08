# Order Lifecycle State API

[English](README.md) | [한국어](README.ko.md)

This example exposes one in-memory order lifecycle through a small Gin HTTP
API. The lifecycle uses `github.com/bluetape4k/bluetape-go/state` to keep the
allowed order transitions explicit and concurrency-safe.

## Example Scenario

The API starts with one `draft` order. A client can submit it, pay it when the
positive-total guard passes, pack it, and ship it into a final state. A client
can also cancel the order while it is still `draft`, `submitted`, or `paid`.
Invalid commands return `409 Conflict` and leave the current state unchanged.

![Order lifecycle scenario](../../docs/images/readme-diagrams/order-lifecycle-state-api-scenario.png)

## State Model

| From | Event | To | Rule |
|---|---|---|---|
| `draft` | `submit` | `submitted` | The order is ready for payment. |
| `submitted` | `pay` | `paid` | The order total must be positive. |
| `paid` | `pack` | `packed` | Fulfillment can prepare the parcel. |
| `packed` | `ship` | `shipped` | `shipped` is final. |
| `draft`, `submitted`, `paid` | `cancel` | `cancelled` | `cancelled` is final. |

`allowed_events` reports registered events for the current state. It does not
evaluate guards. Use `/orders/current/transitions/:event/can` when a caller
needs to evaluate a guard such as the positive-total payment rule.

## Run

```bash
go run ./examples/order-lifecycle-state-api
```

Optional environment variables:

| Variable | Default | Purpose |
|---|---:|---|
| `HTTP_ADDR` | `:8083` | Listening address. |
| `ORDER_ID` | `order-1001` | Example order identifier. |
| `ORDER_TOTAL_CENTS` | `12900` | Example order total used by the payment guard. |

## Endpoints

```bash
curl http://localhost:8083/healthz
curl http://localhost:8083/orders/current
curl http://localhost:8083/orders/current/transitions/pay/can
curl -X POST http://localhost:8083/orders/current/transitions \
  -H 'Content-Type: application/json' \
  -d '{"event":"submit"}'
```

Transition commands return `409 Conflict` when the event is not valid from the
current state, a final state rejects further transitions, a guard rejects the
event, or a concurrent request loses the state-change race. Malformed JSON and
unknown events return `400 Bad Request`.

## When This Is Enough

A finite state machine is enough when the process is short-lived, state changes
are synchronous, and the next legal command can be derived from the current
state. In this example, the API only needs to protect an order from impossible
commands such as paying before submission or cancelling after shipment.

Use a workflow runner instead when the process needs durable timers, human
approvals, retries across service restarts, compensation, or long-running
multi-service orchestration.

The example is intentionally in-memory. It demonstrates lifecycle rules and
HTTP error mapping, not production persistence.

## Architecture

Gin owns HTTP routing and JSON binding. The order handlers translate route
inputs into `state.Machine` calls, and `state.Machine` owns lifecycle legality,
guard evaluation, final-state rejection, and concurrent transition conflicts.
The handler maps package sentinel errors to stable HTTP responses.

![Order lifecycle architecture](../../docs/images/readme-diagrams/order-lifecycle-state-api-architecture.png)

## Sequence Diagram

The main request path is intentionally synchronous: the handler parses the
event, asks the state machine to transition or to check whether a transition can
run, and then writes either a snapshot response or a structured error response.

![Order lifecycle sequence](../../docs/images/readme-diagrams/order-lifecycle-state-api-sequence.png)

## Test

```bash
go test -count=1 ./examples/order-lifecycle-state-api/...
go test -race -count=1 ./examples/order-lifecycle-state-api/...
```
