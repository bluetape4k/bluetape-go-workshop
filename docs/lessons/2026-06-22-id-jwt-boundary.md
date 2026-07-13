# ID and JWT Boundary Example Lessons

## Context

- Issue: #44 `[v0.6.0] Add ID and JWT boundary example`
- Example: `examples/id-jwt-boundary`
- Packages: `github.com/bluetape4k/bluetape-go/id`,
  `github.com/bluetape4k/bluetape-go/jwt`

## Decisions

- Keep the example as a small Gin order intake API instead of a reusable auth
  middleware. The lesson is the application boundary, not a framework.
- Use `jwt.NewFixedHMACProvider` with a committed demo secret only for
  deterministic local commands and tests.
- Use `id.NewUUIDV7Generator` for production wiring and an injected generator
  for deterministic service tests.
- Map upstream JWT parse failures to allowlisted public codes. Expired tokens
  get `expired_token`; signature, malformed, key, issuer, and audience failures
  get `invalid_token`.
- Keep UUID v7 values and JWT bearer tokens conceptually separate in code and
  docs. IDs are not credentials; signed claims are not encrypted.

## Pitfalls

- Importing `bluetape-go/jwt` and `bluetape-go/id` for the first time required
  `go mod tidy` to add transitive `go.sum` checksums for `github.com/golang-jwt/jwt/v5`,
  `github.com/oklog/ulid/v2`, and `github.com/segmentio/ksuid`.
- An expired-token regression test must sign with the same `kid` as the service
  provider. Otherwise key lookup fails first and correctly maps to
  `invalid_token`.
- Public error tests should assert absence of the raw token, demo secret, and
  common parser diagnostics because JWT libraries often return useful but
  boundary-inappropriate error text.

## Verification Notes

- Focused service tests cover valid, missing, expired, malformed, wrong-key,
  forbidden, invalid request, ID shape, and ID generator failure paths.
- Entrypoint tests cover loopback-only bind resolution and HTTP server
  timeouts.
- Live smoke should include:
  - `/healthz`
  - `/tokens`
  - `/orders` with a valid token
  - missing token
  - malformed token
  - forbidden scope
  - expired token

## Follow-Up

- Issue #76 can link this example as the base boundary before adding more
  explicit identifier/key tradeoff comparisons.
- Issue #78 should avoid reimplementing auth. Compose this example with later
  storage/cache/realtime examples through clearly documented trust boundaries.

## Diagram Evidence

- The scenario, architecture, and protected-order sequence are grounded in the
  current example source and are shared by both README locales in the same
  asset order.
- SVG-to-PNG review must verify rendered arrowhead direction, explicit
  per-color marker parity, endpoint contact with the intended boundary at
  native pixels, and full-size readability. A valid SVG alone is not evidence.
- Connector bends reserve at least 24 px of straight terminal distance for the
  marker footprint and enough bend clearance to keep the arrowhead detached
  from the corner. Automated geometry success alone is insufficient.
- JWT failures visibly terminate before UUID generation, so the diagrams never
  imply that identifiers authorize a request.
