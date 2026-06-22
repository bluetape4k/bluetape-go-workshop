# id-jwt-boundary

[English](README.md) | [한국어](README.ko.md)

Gin order intake example for `id` and `jwt`.

The example issues a short-lived local demo JWT, validates it at an HTTP
boundary, and then creates internal UUID v7 order identifiers. It deliberately
keeps identifiers and bearer tokens separate: generated IDs are not secrets,
and JWT claims are signed for integrity but are not encrypted.

## Scenario

A trusted upstream gateway sends an order request to an internal order intake
service. The gateway proves request context with a signed JWT containing
`subject`, `role`, and `scope` claims. The order service verifies issuer,
audience, expiration, role, and scope before generating internal order and
request IDs.

This is not a full auth system. It is the boundary where an application decides
what to trust from a verified token and what it must generate for its own
internal workflow.

## What It Demonstrates

- UUID v7 order/request IDs from `bluetape-go/id`.
- Fixed-HMAC local demo token issue/verify flow from `bluetape-go/jwt`.
- Stable public errors for missing, invalid, expired, and forbidden tokens.
- Secret-safe error responses that do not echo raw tokens, secrets, or parser
  diagnostics.
- Documentation boundaries for signed claims versus encrypted data.

## Run

```bash
go run ./examples/id-jwt-boundary
```

The service listens on `127.0.0.1:8096` by default.

```bash
curl http://127.0.0.1:8096/healthz
```

Issue a local demo token:

```bash
TOKEN=$(
  curl -s -X POST http://127.0.0.1:8096/tokens \
    -H 'Content-Type: application/json' \
    -d '{"subject":"customer-1001","role":"customer","scopes":["orders:create"],"ttl_seconds":900}' \
  | jq -r '.token'
)
```

Create an order with the verified token:

```bash
curl -X POST http://127.0.0.1:8096/orders \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{"sku":"sku-blue-tape","quantity":2}'
```

Try the boundary failures:

```bash
curl -X POST http://127.0.0.1:8096/orders \
  -H 'Content-Type: application/json' \
  -d '{"sku":"sku-blue-tape","quantity":1}'

curl -X POST http://127.0.0.1:8096/orders \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer not-a-jwt' \
  -d '{"sku":"sku-blue-tape","quantity":1}'

READ_TOKEN=$(
  curl -s -X POST http://127.0.0.1:8096/tokens \
    -H 'Content-Type: application/json' \
    -d '{"subject":"customer-1001","role":"customer","scopes":["orders:read"],"ttl_seconds":900}' \
  | jq -r '.token'
)
curl -X POST http://127.0.0.1:8096/orders \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${READ_TOKEN}" \
  -d '{"sku":"sku-blue-tape","quantity":1}'

SHORT_TOKEN=$(
  curl -s -X POST http://127.0.0.1:8096/tokens \
    -H 'Content-Type: application/json' \
    -d '{"subject":"customer-1001","role":"customer","scopes":["orders:create"],"ttl_seconds":1}' \
  | jq -r '.token'
)
sleep 2
curl -X POST http://127.0.0.1:8096/orders \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer ${SHORT_TOKEN}" \
  -d '{"sku":"sku-blue-tape","quantity":1}'
```

Expected public error codes include `missing_token`, `invalid_token`,
`forbidden`, and `expired_token`.

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Process liveness only. |
| `POST` | `/tokens` | Issue a local fixed-HMAC demo token. |
| `POST` | `/orders` | Verify a bearer token and create internal IDs. |

## Boundary Notes

- UUID v7 values are useful sortable identifiers. They are not authorization
  credentials and must not be treated as unguessable secrets.
- JWT signatures protect claim integrity and signing-key possession. They do
  not hide claim values from token holders.
- The fixed HMAC secret in this example is committed only so the demo is
  deterministic and runnable. Real services should load secrets from a secret
  manager or environment, rotate keys, use TLS, and scrub logs.
- Role/scope checks here are intentionally small local policy hooks, not a
  reusable authorization framework.

## Test

```bash
go test -count=1 ./examples/id-jwt-boundary/...
go test -race -count=1 ./examples/id-jwt-boundary/...
```
