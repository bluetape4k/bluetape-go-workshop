# catalog-refresh-resilience

SKU refresh example for `bluetape-go/resilience` retry and timeout policies.

This example models a catalog refresh worker that updates a storefront read
model for one SKU. The upstream catalog provider is allowed to fail once or run
too slowly, but the refresh worker must keep the policy behavior explicit:

- retry transient provider errors without hiding the final error shape;
- give each attempt its own timeout budget;
- emit low-cardinality resilience events that a caller can bridge to logs or
  metrics;
- keep the example as plain domain code instead of another HTTP service.

## Scenario

![Catalog refresh scenario flow](../../docs/images/readme-diagrams/catalog-refresh-scenario-flow.png)

The worker receives one SKU, calls the provider, and writes the refreshed product
projection only when the protected operation succeeds. The `Source` function is
the only dependency the example needs, which keeps the focus on policy wiring.

## Policy Wiring

![Catalog refresh policy sequence](../../docs/images/readme-diagrams/catalog-refresh-policy-sequence.png)

`resilience.Run(ctx, operation, retry, timeout)` applies retry as the outer
policy and timeout as the inner policy. That means every retry attempt creates a
fresh timeout context around the provider call.

```go
product, err := refresher.Refresh(ctx, "sku-1", source)
```

The refresher uses `NoBackoff` so tests remain deterministic. A production job
could choose a constant or exponential backoff without changing the example's
domain boundary.

## Outcomes

![Catalog refresh outcome matrix](../../docs/images/readme-diagrams/catalog-refresh-outcome-matrix.png)

| Case | Behavior | Test |
|---|---|---|
| Transient provider failure | First attempt emits a retry event; second attempt succeeds. | `TestRefreshRetriesTransientFailure` |
| Slow provider | Timeout wraps the provider deadline; retry reports exhaustion after the configured attempt count. | `TestRefreshReportsPolicyTimeout` |
| Invalid input | Blank SKU and nil source fail before policies run. | `TestRefreshRejectsInvalidInput` |

## Run

```bash
go test -count=1 ./examples/catalog-refresh-resilience/...
```
