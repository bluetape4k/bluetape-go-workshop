# Issue #76 Design: Token Refresh and Claims Validation Example

## Frame

- Issue: #76 `[v0.6.0] Add token refresh and claims validation example`
- Milestone: `0.6.0`
- Branch/worktree: `feat/issue-76-token-refresh` under
  `.worktrees/feat-issue-76-token-refresh`.
- Base example: #44 `examples/id-jwt-boundary` already establishes the
  fixed-HMAC JWT boundary, public error allowlist, and signed-not-encrypted
  documentation language.

## Goal

Create `examples/token-refresh-claims`, a runnable Gin example that validates
short-lived access-token claims, rejects expired/malformed/wrong-claim tokens,
and exchanges refresh tokens for new access tokens with clear public errors.

This example teaches the application boundary around access-token and
refresh-token use. It is not an OIDC provider, session store, revocation list,
JWKS endpoint, or reusable auth middleware.

## Evidence

- `gh issue view 76` requires a runnable example, valid/expired/invalid
  claim tests, refresh behavior tests, and README navigation that links #44 as
  the base ID/JWT boundary example.
- `go doc github.com/bluetape4k/bluetape-go/jwt` exposes
  `NewFixedHMACProvider`, `Compose`, `Parse`, `WithAudience`, `WithClaim`,
  `WithExpiresAfter`, `WithJWTID`, `WithExpectedIssuer`,
  `WithExpectedAudience`, `WithExpirationRequired`, `WithParseClock`, and
  `Reader.RemainingTTL`.
- `examples/id-jwt-boundary` maps upstream JWT parse failures to local
  sentinels and keeps public responses free of raw token, secret, and parser
  diagnostics.

## HTTP Contract

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Process liveness only. |
| `POST` | `/sessions` | Issue a local demo access-token and refresh-token pair. |
| `GET` | `/profile` | Validate an access token and return verified claim context. |
| `POST` | `/tokens/refresh` | Validate a refresh token and issue a new access token. |

## JWT Contract

- Use one `jwt.NewFixedHMACProvider(jwt.HS256, secret, ...)` with a deterministic
  demo secret and clock injection.
- Access tokens carry:
  - `iss = token-refresh-claims`
  - `aud = session-api`
  - `sub = request.subject`
  - `token_use = access`
  - `role = customer`
  - `scope = profile:read`
  - `session_id = deterministic generated ID`
  - `jti = deterministic generated ID`
- Refresh tokens carry:
  - `iss = token-refresh-claims`
  - `aud = token-refresh`
  - `sub = request.subject`
  - `token_use = refresh`
  - `session_id = same session id as access token`
  - `jti = deterministic generated ID`
- Parse access and refresh tokens with expected issuer, token-specific
  audience, required expiration, and parse clock.
- Treat `jwt.ErrExpiredToken` as `ErrExpiredToken`. Treat malformed signatures,
  wrong keys, and parser failures as `ErrInvalidToken`. Treat verified but
  wrong claim shape as `ErrInvalidClaims`.

## Error Contract

Public responses use this shape:

```json
{"error_code":"invalid_token","message":"The token could not be verified."}
```

Allowed public codes:

- `invalid_request` (`400`)
- `missing_token` (`401`)
- `expired_token` (`401`)
- `invalid_token` (`401`)
- `invalid_claims` (`403`)
- `internal_error` (`500`)

Responses must not echo raw tokens, the demo secret, or JWT parser text.

## Non-Goals

- No durable session/revocation store.
- No refresh-token reuse detection or rotation family tracking.
- No OIDC discovery, JWKS, or browser cookie flow.
- No new dependencies beyond existing Gin and `bluetape-go/jwt`.

## Documentation Requirements

- Add English/Korean README pair under `examples/token-refresh-claims`.
- Root English/Korean README tables include the example.
- Root run section links #44 as the base ID/JWT boundary example and explains
  that #76 focuses on access/refresh token-use claims.
- README boundary notes state that production refresh-token flows require
  durable session/revocation storage, key rotation, TLS, and secret management.

## Test Requirements

- Valid session issues both token types and `/profile` accepts the access token.
- Expired access token returns `expired_token`.
- Malformed/wrong-key token returns `invalid_token`.
- Verified access token with wrong `aud`, `token_use`, role, or scope returns
  `invalid_claims`.
- Valid refresh token returns a new access token that `/profile` accepts.
- Access token submitted to refresh endpoint is rejected as `invalid_claims`.
- Refresh token submitted as bearer access token is rejected as `invalid_claims`.
- Public error bodies do not leak raw token, demo secret, or parser diagnostics.
- `main.go` keeps loopback binding and bounded server timeouts.

## Completion Criteria

- Issue #76 acceptance criteria are implemented.
- PR body closes #76 and ends with `## DoD Status`.
- PR metadata mirrors the issue: assignee `debop`, milestone `0.6.0`, labels
  `enhancement` and `examples`.
