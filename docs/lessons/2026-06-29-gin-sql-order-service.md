# Gin SQL order service integration example

## Decision

Add `examples/gin-sql-order-service` as the milestone-level composition after
the focused SQL repository, SQL transaction boundary, and Gin SQL CRUD examples.
The example teaches how a Gin HTTP boundary calls a service-owned transaction
that coordinates multiple `sqlkit` repositories for one order aggregate.

## Rationale

Issue #65 should not duplicate #62, #63, or #64. The new reader question is:
what changes when the same order workflow needs HTTP parsing, multi-table
transaction ownership, and reusable SQL repositories at the same time?

The implementation keeps that question visible:

- Gin handlers bind JSON, path parameters, request timeouts, and public error
  codes.
- `Service` owns `sqlkit.WithTx`, ID/time selection, repository call order, and
  commit or rollback.
- `OrderRepository`, `ItemRepository`, and `StatusRepository` each own their
  table SQL and accept `sqlkit` execution/query interfaces.
- A deterministic `reject_after_items` flag proves rollback without pulling in
  payment, inventory, fulfillment, or outbox scope.

## Rejected

- Reusing sibling `internal` packages from prior examples. Their boundaries
  correctly keep each example runnable and inspectable on its own.
- Adding inventory, payment, fulfillment, outbox, auth, or pagination. Those
  would hide the issue #65 lesson behind unrelated production concerns.
- Letting repositories open transactions. That would undo the #63 service
  boundary lesson and make aggregate rollback harder to reason about.

## Verification

- `go run ./examples/gin-sql-order-service`
- `go test -count=1 ./examples/gin-sql-order-service/...`
- `go test -race -count=1 ./examples/gin-sql-order-service/...`
- `golangci-lint cache clean && make ci`
- `xmllint --noout` on all Gin SQL order service diagrams
- CairoSVG render for all Gin SQL order service PNG diagrams
- PNG inspection for clipping, overlap, marker color, and label readability
