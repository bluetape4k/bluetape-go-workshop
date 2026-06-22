# Checkout Guard Integration Lesson

## Summary

`examples/checkout-guard-integration` composes the v0.6.0 focused utility
examples into one checkout submission boundary: ID generation, JWT claim
verification, money/rule pricing, and probabilistic repeated-submission
admission.

## Decisions

- Keep token issue local and demo-only. The lesson is the protected checkout
  boundary, not identity-provider design.
- Require `token_use`, issuer, audience, role, scope, and session ID before
  pricing or admission.
- Keep pricing single-currency. Mixed line currencies are rejected instead of
  converted.
- Use local rule functions for `vip-service-discount`, `regional-vat`, and
  `restricted-category`; no generic rule engine is introduced.
- Admit idempotency keys through a Bloom filter only after request validation
  and rule pricing succeed.
- Document `probably_seen` as a possible duplicate or false positive, not
  authoritative proof.

## Boundaries

- No payment capture, inventory reservation, fulfillment workflow, ledger, or
  persistent checkout store.
- No token refresh flow; `token-refresh-claims` owns that focused lesson.
- No multi-currency conversion; `multi-currency-invoice-rules` owns grouped
  currency totals.

## Verification

```bash
go test -count=1 ./examples/checkout-guard-integration/...
go test -race -count=1 ./examples/checkout-guard-integration/...
```
