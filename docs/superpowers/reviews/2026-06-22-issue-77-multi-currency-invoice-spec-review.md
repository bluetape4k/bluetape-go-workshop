# Issue #77 Spec Review

## Verdict

Proceed with implementation after keeping the example local and narrow. The
scope is consistent with #77 because it composes `money` with rule-style
decisions while avoiding a new generic rule engine.

## Six-Lane Review

| Lane | Concern | Decision |
|---|---|---|
| Correctness | Multi-currency arithmetic | Group by currency and never add/subtract across currency boundaries. |
| Domain | Tax-like rules | Keep `regional-vat` explicitly illustrative, not compliance language. |
| Security | HTTP boundary | Cap JSON body, disable trusted proxies, and avoid raw parser diagnostics in errors. |
| Stability | Determinism | Sort currency totals and rule decisions for stable tests and docs. |
| Developer/API | Reuse | Reuse `money.Money`; keep rules example-local because no reusable rule package exists. |
| User/Docs | Navigation | Link #45 as the base single-currency money/rule pricing lesson. |

## Required Adjustments

- Name the example `multi-currency-invoice-rules` to keep the issue's scope
  visible in paths and README tables.
- Include one zero-minor-unit rounding test, preferably JPY, so currency scale
  is visible beyond USD/EUR.
- Keep all request amount fields as strings and avoid `float64` arithmetic.
- Document that no exchange-rate conversion occurs.

## Blockers

None.
