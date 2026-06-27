# Code review: issue #113 shared Redis Bloom admission

## Scope

- New runnable example: `examples/shared-redis-bloom-admission`
- New README diagrams for architecture, cross-instance sequence, and decision
  policy
- Root README navigation and focused run instructions
- Lesson note for the Redis-backed Bloom admission example

## Findings

No P0/P1 findings in the local review pass.

## Checks

- `probably_seen` is never described as authorization or exact dedupe.
- First insert, repeated event, and cross-instance shared-state behavior are
  covered by Redis-backed tests.
- Config fingerprint mismatch is surfaced as `filter_config_mismatch` rather
  than silently mixing incompatible Bloom sizing.
- Redis outage and canceled requests map to `filter_unavailable`.
- The HTTP server binds to loopback by default and rejects non-loopback
  `HTTP_ADDR` values.
- README diagrams render as PNG and keep architecture, sequence, and policy
  concerns separated.

## Residual risk

The runtime example expects an externally supplied Redis instance for manual
`go run`. Test coverage uses Testcontainers Redis, but the example does not
include a bundled docker-compose file because repository examples already share
Testcontainers fixtures for integration verification.
