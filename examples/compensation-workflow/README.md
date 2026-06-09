# Compensation Workflow

[English](README.md) | [한국어](README.ko.md)

This example exposes a request-scoped fulfillment workflow through a small Gin
HTTP API. It uses `github.com/bluetape4k/bluetape-go/workflow` to run ordered
forward steps, and then runs application-owned compensation steps in reverse
order when a later forward step fails.

## Example Scenario

The API accepts one fulfillment request. The workflow reserves inventory,
authorizes payment, and creates a shipment. Inventory and payment are
reversible side effects, so each successful forward step registers a
compensation handler. If shipment creation fails, the API voids payment first,
then releases inventory, and still returns the original shipment error to the
caller.

![Compensation workflow scenario](../../docs/images/readme-diagrams/compensation-workflow-scenario.png)

## Workflow Steps

| Step | Runner | Boundary | Failure behavior |
|---|---|---|---|
| `reserve-inventory` | `workflow.Sequential` child | Reserves stock for the order and registers `release-inventory`. | Stops before payment when stock is unavailable. |
| `authorize-payment` | `workflow.Sequential` child | Authorizes payment and registers `void-payment`. | Runs `release-inventory` because inventory was already reserved. |
| `create-shipment` | `workflow.Sequential` child | Creates the shipment after reversible steps complete. | Runs `void-payment`, then `release-inventory`. |
| `compensation` | `workflow.Sequential` with `ContinueOnFailure` | Runs registered compensation handlers in reverse order. | Keeps running later compensation steps and preserves the original forward error. |

The HTTP handler projects `workreport.Report` into stable JSON and deliberately
omits runtime timestamps, so tests can assert the forward and compensation
execution tree without timing noise.

## Run

```bash
go run ./examples/compensation-workflow
```

Optional environment variables:

| Variable | Default | Purpose |
|---|---:|---|
| `HTTP_ADDR` | `:8087` | Listening address. |

## Endpoints

```bash
curl http://localhost:8087/healthz
curl -X POST http://localhost:8087/compensation/fulfillment \
  -H 'Content-Type: application/json' \
  -d '{
    "order_id": "order-1001",
    "stock_available": true,
    "payment_authorized": true,
    "shipment_provider_available": true
  }'
```

Useful variants:

```bash
# Shipment failure triggers void-payment, then release-inventory.
curl -X POST http://localhost:8087/compensation/fulfillment \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"order-1002","stock_available":true,"payment_authorized":true,"shipment_provider_available":false}'

# Compensation failure preserves the original shipment error and continues.
curl -X POST http://localhost:8087/compensation/fulfillment \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"order-1003","stock_available":true,"payment_authorized":true,"shipment_provider_available":false,"void_payment_fails":true}'
```

Successful reports return `200 OK`. Failed forward reports return
`409 Conflict`. If compensation runs, the response sets `compensated=true` and
keeps `original_error` from the forward workflow. Cancelled workflow reports
return `408 Request Timeout`. Malformed JSON, missing `order_id`, and blank
`order_id` return `400 Bad Request`.

## Compensation vs State Transition

A state machine example focuses on whether one command can legally move from
one state to another. This example focuses on side effects that already
happened. The forward workflow records which reversible side effects succeeded,
then the compensation workflow uses that stack to clean up in reverse order.

This is still a request-scoped teaching example. Use durable storage, idempotent
external commands, retry policy, and an outbox or workflow engine when
compensation must survive process restarts or cross service boundaries.

## Architecture

Gin owns routing, JSON binding, and HTTP status mapping. The request handler
builds a fresh run object per request so each run owns its own side-effect flags
and compensation stack. The `workflow` package owns execution order, while
`workreport` owns the forward and compensation result trees.

![Compensation workflow architecture](../../docs/images/readme-diagrams/compensation-workflow-architecture.png)

## Sequence Diagram

The main request path is synchronous. The handler runs the forward workflow; a
failed forward report triggers the reverse compensation workflow, and the
response mapper returns both trees while preserving the original error.

![Compensation workflow sequence](../../docs/images/readme-diagrams/compensation-workflow-sequence.png)

## Test

```bash
go test -count=1 ./examples/compensation-workflow/...
go test -race -count=1 ./examples/compensation-workflow/...
```
