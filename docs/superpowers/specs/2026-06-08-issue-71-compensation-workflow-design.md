# Issue #71 Design: Compensation Workflow Example

## Goal

Add a focused v0.4.0 example that demonstrates ordered workflow execution plus
rollback-style compensation after a later fulfillment step fails.

## Non-Goals

- Do not implement a durable saga coordinator, queue, database, retry scheduler,
  or external inventory/payment/shipping service.
- Do not claim `workflow` provides compensation by itself.
- Do not duplicate the broader fulfillment runner example from #39.
- Do not hide the original forward failure behind compensation reports.

## Example

- Path: `examples/compensation-workflow`
- Package: `internal/compensation`
- HTTP framework: Gin
- Default port: `:8087`

## Scenario

A fulfillment workflow executes three forward steps:

1. `reserve-inventory`
2. `authorize-payment`
3. `create-shipment`

The first two steps are reversible. When `create-shipment` fails after inventory
and payment succeeded, the example runs compensation in reverse order:

1. `void-payment`
2. `release-inventory`

The response preserves the original shipment error and also exposes a
compensation report tree.

## API

### `GET /healthz`

Returns `200 OK` with `{"status":"ok"}`.

### `POST /compensation/fulfillment`

Request:

```json
{
  "order_id": "order-1001",
  "stock_available": true,
  "payment_authorized": true,
  "shipment_provider_available": false,
  "void_payment_fails": false,
  "release_inventory_fails": false
}
```

Rules:

- `order_id` is required after trimming whitespace.
- `stock_available=false` fails before registering inventory compensation.
- `payment_authorized=false` fails after inventory reservation and then releases
  inventory.
- `shipment_provider_available=false` fails after inventory reservation and
  payment authorization, then voids payment and releases inventory.
- `void_payment_fails=true` records a failed payment compensation but still runs
  inventory release.
- `release_inventory_fails=true` records a failed inventory compensation.
- Malformed JSON and invalid request fields return `400 Bad Request`.
- Caller cancellation maps to `408 Request Timeout`.

Response:

```json
{
  "order_id": "order-1001",
  "completed": false,
  "compensated": true,
  "original_error": "shipment provider is unavailable",
  "effects": {
    "inventory_reserved": false,
    "payment_authorized": false,
    "shipment_created": false
  },
  "report": {
    "name": "compensating-fulfillment",
    "status": "failed",
    "children": []
  }
}
```

## Report Shape

Expose a stable DTO:

- `name`
- `status`
- `error`
- `reason`
- `success`
- `failure`
- `partial`
- `cancelled`
- `children`

Timestamps are intentionally omitted.

## Design

`compensationRun` owns request-scoped side-effect flags and a stack of
compensation handlers. The forward runner is built as:

```go
workflow.Sequential(
    "fulfillment-forward",
    workreport.StopOnFailure,
    r.reserveInventory,
    r.authorizePayment,
    r.createShipment,
)
```

Successful reversible steps append their compensation work to the stack. If the
forward report is not successful, compensation handlers are copied in reverse
order and executed through:

```go
workflow.Sequential("compensation", workreport.ContinueOnFailure, works...)
```

The top-level report keeps the original forward error as its own error, and
embeds both forward and compensation child reports.

## HTTP Status Mapping

- completed workflow -> `200 OK`
- compensated forward failure -> `409 Conflict`
- cancellation -> `408 Request Timeout`
- invalid JSON/request -> `400 Bad Request`
- unexpected report state -> `500 Internal Server Error`

## Tests

Focused tests must cover:

- health endpoint
- successful execution with no compensation
- shipment failure runs `void-payment` then `release-inventory`
- payment failure runs only `release-inventory`
- compensation failure still preserves the original shipment error and continues
  remaining compensation
- inventory failure has no compensation work
- caller cancellation maps to `408`
- malformed JSON and invalid request fields return `400`
- race test over the example package

## Documentation and Diagrams

Add English and Korean README files with:

- Example Scenario
- when compensation is different from a plain state transition
- API and response examples
- Architecture
- Sequence Diagram
- production hardening notes

Generate README diagram assets under `docs/images/readme-diagrams/`:

- `compensation-workflow-scenario`
- `compensation-workflow-architecture`
- `compensation-workflow-sequence`

README files embed PNG only and keep generated diagram labels in English.

