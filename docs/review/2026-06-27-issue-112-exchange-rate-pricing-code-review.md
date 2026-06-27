# Code review: issue #112 exchange-rate pricing

## Scope

- New runnable example: `examples/exchange-rate-pricing`
- New README diagrams for architecture, request sequence, and stale quote policy
- Root README navigation and focused run instructions
- Lesson note for the provider-backed money conversion example

## Findings

No P0/P1 findings in the local review pass.

## Checks

- Provider I/O is isolated behind `money.ExchangeRateProvider` and receives a
  child context deadline.
- Mixed line currencies are rejected before provider I/O.
- Language-only or unsupported locales map to stable `invalid_locale` HTTP
  errors.
- Provider failure details are not exposed in HTTP error responses.
- Stale fallback is visible and requires explicit `allow_stale_quote` opt-in.
- Tests cover fresh quotes, stale opt-in, stale rejection, provider failure,
  unsupported locale, currency mismatch, provider deadline propagation, and
  router error mapping.

## Residual risk

The runtime example intentionally uses a deterministic static provider. Live ECB
or IMF provider wiring is left for a separate integration-oriented example if a
future milestone needs network-backed provider behavior.
