# Token Refresh Claims Example Lessons

## Context

- Issue: #76 `[v0.6.0] Add token refresh and claims validation example`
- Example: `examples/token-refresh-claims`
- Dependency focus: `github.com/bluetape4k/bluetape-go/jwt`
- Base lesson: #44 `examples/id-jwt-boundary`

## Lessons

- Keep access-token and refresh-token contracts explicit. This example uses
  different `aud` values and a `token_use` claim so a verified refresh token is
  still rejected at a protected resource.
- Parse enough of the token to verify signature, issuer, and expiration, then
  map operation-specific claim mismatches to a local `invalid_claims` public
  code. This keeps wrong audience or wrong token-use separate from malformed or
  unverifiable tokens.
- A stateless refresh exchange is useful for a workshop boundary but must not
  be described as production session management. Durable session storage,
  revocation, reuse detection, key rotation, TLS, and log scrubbing remain
  production responsibilities.
- Keep public error responses allowlisted. Raw bearer values, demo secrets, and
  parser diagnostics are useful for tests and logs but not for callers.

## Verification Commands

```bash
go test -count=1 ./examples/token-refresh-claims/...
go test -race -count=1 ./examples/token-refresh-claims/...
```
