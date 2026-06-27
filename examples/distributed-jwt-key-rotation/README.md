# distributed-jwt-key-rotation

[English](README.md) | [한국어](README.ko.md)

This example shows how a small API can issue and verify JWTs when several
instances share signing keys through Redis. It combines `bluetape-go/jwt`,
`jwt/redis`, and `cache` so the reader can see the production shape behind key
rotation, retained keys, reader caching, and request-scoped Redis timeouts.

The important lesson is not just "put keys in Redis". The service must let a
new instance verify tokens issued elsewhere, keep old tokens valid while their
signing key is retained, and still fail closed when a cached reader refers to a
key that is no longer trusted.

## What this example teaches

- Build a repository-backed `jwt.DistributedHMACProvider` with
  `jwt/redis.New`.
- Rotate the current `kid` without breaking tokens signed by retained keys.
- Add a local `cache.Memory[string,*jwt.Reader]` for hot verification paths.
- Revalidate the token `kid` against Redis even when the local reader cache is
  warm.
- Keep Redis calls under a short child `context.Context`, so caller
  cancellation and deadlines remain visible.

## Scenario

Imagine two API instances behind a load balancer. `api-a` issues a token, then
`api-b` receives the next request with that token. Both instances use the same
Redis namespace, so they agree on the current key and the retained key set.

![Distributed JWT key rotation architecture](../../docs/images/readme-diagrams/distributed-jwt-key-rotation-architecture.png)

The example keeps signing material in Redis and keeps only verified readers in
the local process cache. That split matters: the cache accelerates repeated
verification, but Redis remains the source of truth for whether a `kid` is still
trusted.

## Request flow

The service exposes three endpoints:

| Endpoint | Purpose |
|---|---|
| `POST /tokens` | Issue an access token with the repository-backed current HMAC key. |
| `GET /profile` | Verify a bearer token and return the subject/scopes carried by the token. |
| `POST /keys/rotate` | Force a new current key into Redis and retain the old key until its TTL window ends. |

![Distributed JWT issue verify rotate sequence](../../docs/images/readme-diagrams/distributed-jwt-key-rotation-sequence.png)

`Service.IssueToken` signs with the current `kid`. `Service.RotateKey` makes a
new `kid` current, but the previous key stays in the repository long enough for
already-issued tokens to keep working. `Service.VerifyToken` parses the token
through `NewCachedDistributedProvider`, which means repeated reads can be fast
without trusting stale key state.

## Why cached verification stays safe

The reader cache stores parse readers, not signing keys. A cache hit is accepted
only after the distributed provider checks that the token's `kid` is still
present in Redis.

![Distributed JWT cached verification revalidation](../../docs/images/readme-diagrams/distributed-jwt-key-rotation-cache-revalidation.png)

This gives the example its intended tradeoff: the hot path avoids rebuilding the
same reader repeatedly, but removal or expiry of a retained key still causes
verification to fail instead of silently accepting stale trust.

## Code map

| File or type | What to read for |
|---|---|
| `main.go` | Runtime wiring: Redis client, `NODE_ID`, `HTTP_ADDR`, and the HTTP server. |
| `internal/distributedjwt/service.go` | Provider setup, issue/verify/rotate operations, and endpoint handlers. |
| `Service.operationContext` | The short child context used before repository I/O. |
| `Service.VerifyToken` | Cached distributed verification and public error mapping. |
| `internal/distributedjwt/service_test.go` | Cross-instance sharing, rotation retention, stale token rejection, cancellation, and cache revalidation tests. |

## Outcomes

| Case | Behavior | Test |
|---|---|---|
| Shared keys | Two services constructed with the same namespace verify each other's tokens. | `TestServiceSharesRotatedKeysAcrossInstances` |
| Rotation retention | A forced rotation changes the current `kid`, while the previous token remains valid inside the retained key window. | `TestServiceSharesRotatedKeysAcrossInstances` |
| Unknown or stale tokens | Tokens signed under another namespace and expired tokens map to stable public errors. | `TestServiceRejectsUnknownKIDAndExpiredToken` |
| Context budget | Cancelled contexts fail before Redis I/O can hide the caller's cancellation. | `TestServiceHonorsCancelledContextBeforeRepositoryIO` |
| Cached verification | Warm verification stays stable after rotation because cached readers revalidate the retained key. | `TestRouterUsesCachedProviderWithKeyRevalidation` |

## Run

Start Redis locally, then run one service instance:

```bash
export REDIS_ADDR=localhost:6379
go run ./examples/distributed-jwt-key-rotation
```

To see the distributed shape more clearly, run two instances in separate
terminals:

```bash
NODE_ID=api-a HTTP_ADDR=127.0.0.1:8099 go run ./examples/distributed-jwt-key-rotation
NODE_ID=api-b HTTP_ADDR=127.0.0.1:8100 go run ./examples/distributed-jwt-key-rotation
```

Issue a token from `api-a`, verify it through `api-b`, rotate keys, then verify
the original token again while the retained key is still valid:

```bash
TOKEN=$(curl -s http://127.0.0.1:8099/tokens \
  -H 'Content-Type: application/json' \
  -d '{"subject":"customer-42","scopes":["orders:read"],"ttl_seconds":300}' |
  jq -r .access_token)

curl -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8100/profile
curl -X POST http://127.0.0.1:8099/keys/rotate
curl -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8100/profile
```

The tests start Redis with the repository Testcontainers fixture:

```bash
go test -count=1 ./examples/distributed-jwt-key-rotation/...
go test -race -count=1 ./examples/distributed-jwt-key-rotation/...
```
