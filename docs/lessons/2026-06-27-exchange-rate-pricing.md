# Provider-backed exchange-rate pricing example

Issue: #112

## Decision

Use `money.CurrencyByLocale` and `money.ConvertWithProvider` directly in a
runnable Gin display-pricing example, with a deterministic demo provider and
deterministic provider doubles in tests.

## Why

The lesson is the boundary between application pricing and provider-backed
exchange-rate conversion. The app owns subtotal validation, locale selection,
provider I/O deadlines, and stale quote policy. `bluetape-go/money` owns
currency parsing, locale currency defaults, conversion validation, and quote
metadata.

## Verification shape

- Service tests assert fresh conversion, stale opt-in, stale rejection, provider
  failure mapping, unsupported locale handling, and currency mismatch before
  provider I/O.
- Router tests assert public error codes and that provider details do not leak
  into HTTP failure responses.
- README diagrams show architecture, request sequence, and freshness/failure
  policy separately so readers can follow the conversion boundary without
  reading tests first.
