# Money Rule Pricing Lesson

Issue: #45

`examples/money-rule-pricing` demonstrates cart pricing with
`github.com/bluetape4k/bluetape-go/money`.

## Decision

Use `money.Money` for all arithmetic and keep rule primitives local to the
example. The current `bluetape-go` dependency exposes a money package but not a
general rule-engine package, so a reusable rule abstraction would be extra
framework work outside the workshop issue.

## Boundaries

- Public monetary values are strings plus explicit currency.
- Item currency must match the cart currency before arithmetic starts.
- Line totals, discount totals, and final totals are rounded at the cart
  boundary.
- Rejected pricing rules are visible in `rules[]` when the cart is otherwise
  valid.
- Invalid money or mixed currencies are public HTTP errors.

## Follow-Up

Issue #77 can build multi-currency invoice rule evaluation on this baseline by
adding explicit invoice-line grouping and conversion or no-conversion policy
decisions instead of re-teaching basic cart money arithmetic.
