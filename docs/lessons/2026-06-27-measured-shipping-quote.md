# Measured shipping quote example

Issue: #115

## Decision

Use a runnable Gin shipping quote API to teach `measure` parsing, formatting,
derived area/volume, dimensional weight, billable-weight selection, and stable
HTTP error mapping.

## Why

The `measure` package is easiest to understand when a caller sends real unit
strings across an application boundary. Shipping quotes make the unit risk
concrete: dimensions and weight can arrive in metric or imperial forms, but the
application must compare typed values instead of mixing raw floats and suffix
strings.

The example intentionally returns billable measurements instead of money totals.
That keeps dimensional math separate from currency policy and makes the
`measure` lesson the main reader contract.

## Verification shape

- Service tests assert metric input, imperial dimensions with pounds, typed
  incompatible-unit errors, parse failures, divide-by-zero preservation,
  declared max-side validation, and invalid display-unit handling.
- Router tests assert stable JSON responses, malformed JSON handling,
  invalid-measure mapping, incompatible-unit mapping, invalid dimensional
  divisor mapping, max-side request validation, oversize request rejection, and
  health output.
- README diagrams show the architecture, request sequence, and error policy as
  separate reader questions so the measurement boundary is visible without
  hiding HTTP behavior.
