# Issue #72 Design: Order Fulfillment Workflow Integration Example

## Goal

Add the milestone-level 0.4.0 integration example that combines order lifecycle
state transitions, fulfillment workflow execution, report projection, failure
policy, and compensation into one runnable Gin API.

## Non-Goals

- Do not add a durable workflow engine, saga coordinator, queue, database,
  retry scheduler, or external inventory/payment/shipping service.
- Do not import previous example packages as implementation dependencies.
- Do not hide `state`, `workflow`, or `workreport` behind a generic framework.
- Do not claim request-scoped compensation is production-grade durability.

## Example

- Path: `examples/order-fulfillment-integration`
- Package: `internal/orderfulfillment`
- HTTP framework: Gin
- Default port: `:8088`

## Scenario

The API accepts an order fulfillment request and runs one request-scoped
fulfillment:

1. `submit-order` transitions lifecycle from `draft` to `submitted`.
2. `reserve-inventory` records an inventory side effect and registers
   `release-inventory`.
3. `authorize-payment` transitions lifecycle from `submitted` to `paid`,
   records a payment side effect, and registers `void-payment`.
4. `pack-order` transitions lifecycle from `paid` to `packed`.
5. `create-shipment` transitions lifecycle from `packed` to `shipped` when the
   shipment provider is available.

When a later step fails after reversible side effects, the example runs
registered compensation in reverse order and transitions the lifecycle to
`cancelled` when that transition is legal. The response preserves the original
failure and includes both the forward and compensation report trees.

## Lifecycle Model

States:

- `draft`
- `submitted`
- `paid`
- `packed`
- `shipped`
- `cancelled`

Events:

- `submit`: `draft -> submitted`
- `pay`: `submitted -> paid`
- `pack`: `paid -> packed`
- `ship`: `packed -> shipped`
- `cancel`: `draft|submitted|paid|packed -> cancelled`

Final states:

- `shipped`
- `cancelled`

## API

### `GET /healthz`

Returns `200 OK` with `{"status":"ok"}`.

### `POST /orders/fulfillment`

Request:

```json
{
  "order_id": "order-1001",
  "total_cents": 2599,
  "stock_available": true,
  "payment_authorized": true,
  "shipment_provider_available": true,
  "force_invalid_transition": false,
  "void_payment_fails": false,
  "release_inventory_fails": false
}
```

Validation:

- `order_id` is required after trimming whitespace.
- `total_cents` must be positive.
- Malformed JSON and invalid fields return `400 Bad Request`.

Scenario flags:

- `stock_available=false` fails before inventory side effects.
- `payment_authorized=false` fails after inventory reservation and then releases
  inventory.
- `shipment_provider_available=false` fails after inventory reservation and
  payment authorization, then voids payment and releases inventory.
- `force_invalid_transition=true` deliberately attempts `ship` before `pack`
  after payment authorization to demonstrate a state-machine invalid transition
  inside a workflow.
- `void_payment_fails=true` records a failed payment compensation while still
  running inventory release.
- `release_inventory_fails=true` records a failed inventory compensation.

Response:

```json
{
  "order_id": "order-1001",
  "state": "shipped",
  "state_history": ["draft", "submitted", "paid", "packed", "shipped"],
  "completed": true,
  "compensated": false,
  "original_error": "",
  "effects": {
    "inventory_reserved": true,
    "payment_authorized": true,
    "shipment_created": true
  },
  "summary": {
    "completed": 5,
    "failed": 0,
    "partial": 0,
    "cancelled": 0,
    "total": 5,
    "success": true,
    "failure": false
  },
  "report": {
    "name": "order-fulfillment",
    "status": "completed",
    "children": []
  }
}
```

Report projection is stable and omits runtime timestamps.

## HTTP Status Mapping

- successful fulfillment -> `200 OK`
- invalid transition, domain failure, or compensated failure -> `409 Conflict`
- caller cancellation -> `408 Request Timeout`
- invalid request -> `400 Bad Request`
- unexpected report state -> `500 Internal Server Error`

## Design

`orderRun` owns one request-scoped state machine, side-effect flags, state
history, and compensation stack.

Forward runner:

```go
workflow.Sequential(
    "order-fulfillment",
    workreport.StopOnFailure,
    r.submitOrder,
    r.reserveInventory,
    r.authorizePayment,
    r.packOrder,
    r.createShipment,
)
```

Compensation runner:

```go
workflow.Sequential(
    "compensation",
    workreport.ContinueOnFailure,
    r.reverseCompensations()...,
)
```

The top-level response uses `workreport` status predicates and a summary helper
like `operations-report-policy`, but does not expose mutable runtime timestamp
fields.

## Diagrams

Generate README diagram assets under `docs/images/readme-diagrams/`:

- `order-fulfillment-integration-scenario`
- `order-fulfillment-integration-architecture`
- `order-fulfillment-integration-sequence`

README files embed PNG only. SVG files remain next to PNGs for review.
Graphviz `.dot`, `.plain`, `*-graphviz.svg`, and `*-graphviz.png` remain route
evidence. Final README SVG/PNG assets must use the existing decorated workshop
baseline, not raw Graphviz output.

The diagram generator must print concrete geometry evidence including
`margins=L/R/T/B`.

## Tests

Focused tests must cover:

- health endpoint
- happy path reaches `shipped`, records all expected lifecycle states, and has a
  successful report summary
- invalid transition scenario returns `409`, preserves the state-machine error,
  and exposes the failed report node
- shipment-provider failure returns `409`, runs `void-payment` then
  `release-inventory`, transitions lifecycle to `cancelled`, and preserves the
  original shipment error
- compensation failure still preserves the original shipment error and reports
  compensation child failures
- caller cancellation before or during fulfillment maps to `408` and, when a
  reversible side effect was already registered, still runs compensation before
  returning
- malformed JSON, blank `order_id`, and non-positive total return `400`
- parallel HTTP requests keep independent state and side-effect flags
- race test over `./examples/order-fulfillment-integration/...`

## Documentation

Add English and Korean README files that include:

- Example Scenario
- how this integrates the smaller 0.4.0 examples
- API and response examples
- Architecture
- Sequence Diagram
- production hardening notes for durability, idempotency, retries, audit, and
  external service integration

Update root `README.md` and `README.ko.md` example tables, quickstart section,
and 0.4.0 roadmap row.
