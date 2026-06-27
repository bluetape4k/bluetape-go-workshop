# Code review: issue #115 measured shipping quote

## Scope

- New runnable example: `examples/measured-shipping-quote`
- New README diagrams for architecture, request sequence, and error policy
- Root README navigation and focused run instructions
- Lesson note for `measure` typed-unit boundary handling

## Findings

No P0/P1 findings in the local review pass.

## Checks

- Caller-facing length strings use built-in `measure` length units through
  `measure.ParseLength`.
- Mass parsing keeps built-in `g`, `kg`, and `ton` while adding local `lb` for
  realistic shipping input.
- Area and volume are derived through `measure.AreaFromLength` and
  `measure.VolumeFromAreaLength`, not untyped multiplication in application
  code.
- Dimensional weight conversion preserves `measure.ErrDivideByZero` through
  `errors.Is` when a divisor reaches the divide operation.
- Public HTTP error codes distinguish invalid request, invalid measurement,
  incompatible unit, and invalid dimensional divisor without echoing raw invalid
  measurement input.
- The runtime HTTP server binds to loopback by default and rejects non-loopback
  `HTTP_ADDR` values.
- README diagrams render as PNG and keep architecture, sequence, and policy
  concerns separated.

## Residual risk

The example does not model carrier money pricing, carrier-specific divisor
tables, insurance, or customs rules. Those are documented as downstream policy
concerns so the example stays focused on `measure`.

## P0/P1 Gate

P0=0 P1=0
