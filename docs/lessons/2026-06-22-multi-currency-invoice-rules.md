# Multi-Currency Invoice Rule Lesson

## Summary

`examples/multi-currency-invoice-rules` extends the #45
`examples/money-rule-pricing` baseline from single-currency cart totals to
invoice totals grouped by currency.

## Decisions

- Keep every monetary amount as a string plus explicit currency in public JSON.
- Use `money.Money` for parsing, multiplication, addition, subtraction, and
  currency-scale rounding.
- Group invoice totals by currency and set `conversion_applied` to `false`.
- Keep `vip-service-discount` and `regional-vat` as example-local rule decision
  functions because the current `bluetape-go` surface does not expose a generic
  rule engine.
- Make zero-minor-unit rounding visible with JPY.

## Boundaries

- No exchange rates or cross-currency settlement.
- `regional-vat` is illustrative, not tax compliance logic.
- Rule decisions are public evidence for the workshop, not a reusable promotion
  framework.

## Verification

```bash
go test -count=1 ./examples/multi-currency-invoice-rules/...
go test -race -count=1 ./examples/multi-currency-invoice-rules/...
```
