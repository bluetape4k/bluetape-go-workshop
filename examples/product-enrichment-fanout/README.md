# product-enrichment-fanout

Product detail enrichment example for `concurrency` and
`testing/concurrency`.

The example fans out pricing, inventory, recommendation, and review-summary
lookups under one request context. Required-provider failure cancels remaining
work; optional-provider failure is recorded without failing the request.
