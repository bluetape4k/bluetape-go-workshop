# token-refresh-claims

[English](README.md) | [한국어](README.ko.md)

Gin token boundary example for `jwt`.

This example issues a short-lived access token plus a longer-lived refresh
token, validates access-token claims at a protected HTTP boundary, and exchanges
a refresh token for a new access token. It builds on the base ID/JWT trust
boundary from [issue #44](../../docs/lessons/2026-06-22-id-jwt-boundary.md).

## Scenario

A trusted session service issues two signed demo tokens. Callers use the access
token at `/profile`. When the access token is close to expiring, callers submit
the refresh token to `/tokens/refresh` and receive a new access token.

The example keeps the access and refresh contracts separate with `aud` and
`token_use` claims. A refresh token cannot call `/profile`, and an access token
cannot be exchanged at `/tokens/refresh`.

This is not a complete auth system. It is a focused boundary for deciding which
verified token claims an application accepts for each operation.

## What It Demonstrates

- Fixed-HMAC local demo access and refresh tokens from `bluetape-go/jwt`.
- Claim validation for issuer, audience, expiration, `token_use`, role, scope,
  subject, and `session_id`.
- Stable public errors for missing, invalid, expired, and wrong-claim tokens.
- Secret-safe error responses that do not echo raw tokens, secrets, or parser
  diagnostics.
- Documentation boundaries for a stateless demo versus production refresh-token
  storage and revocation.

## Run

```bash
go run ./examples/token-refresh-claims
```

The service listens on `127.0.0.1:8097` by default.

```bash
curl http://127.0.0.1:8097/healthz
```

Issue a local demo session:

```bash
SESSION=$(
  curl -s -X POST http://127.0.0.1:8097/sessions \
    -H 'Content-Type: application/json' \
    -d '{"subject":"customer-1001","role":"customer","scopes":["profile:read"],"ttl_seconds":300}'
)
ACCESS_TOKEN=$(printf '%s' "${SESSION}" | jq -r '.access_token')
REFRESH_TOKEN=$(printf '%s' "${SESSION}" | jq -r '.refresh_token')
```

Read a protected profile with the access token:

```bash
curl http://127.0.0.1:8097/profile \
  -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

Exchange the refresh token:

```bash
NEW_ACCESS_TOKEN=$(
  curl -s -X POST http://127.0.0.1:8097/tokens/refresh \
    -H 'Content-Type: application/json' \
    -d "{\"refresh_token\":\"${REFRESH_TOKEN}\"}" \
  | jq -r '.access_token'
)
curl http://127.0.0.1:8097/profile \
  -H "Authorization: Bearer ${NEW_ACCESS_TOKEN}"
```

Try the boundary failures:

```bash
curl http://127.0.0.1:8097/profile

curl http://127.0.0.1:8097/profile \
  -H 'Authorization: Bearer not-a-jwt'

curl http://127.0.0.1:8097/profile \
  -H "Authorization: Bearer ${REFRESH_TOKEN}"

curl -X POST http://127.0.0.1:8097/tokens/refresh \
  -H 'Content-Type: application/json' \
  -d "{\"refresh_token\":\"${ACCESS_TOKEN}\"}"
```

Expected public error codes include `missing_token`, `invalid_token`,
`invalid_claims`, and `expired_token`.

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Process liveness only. |
| `POST` | `/sessions` | Issue local fixed-HMAC access and refresh demo tokens. |
| `GET` | `/profile` | Validate access-token claims and return verified context. |
| `POST` | `/tokens/refresh` | Validate refresh-token claims and issue a new access token. |

## Boundary Notes

- Access and refresh tokens are both signed JWTs in this demo, but they have
  different audiences and `token_use` values.
- JWT signatures protect claim integrity and signing-key possession. They do
  not hide claim values from token holders.
- This example is stateless. It does not detect refresh-token reuse, revoke
  token families, or persist sessions.
- Production refresh flows need durable session/revocation storage, TLS, key
  rotation, secret management, log scrubbing, and replay monitoring.
- See `examples/id-jwt-boundary` for the base #44 lesson on generated IDs,
  bearer tokens, and signed-but-not-encrypted claims.

## Test

```bash
go test -count=1 ./examples/token-refresh-claims/...
go test -race -count=1 ./examples/token-refresh-claims/...
```
