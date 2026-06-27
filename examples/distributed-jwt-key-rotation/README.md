# distributed-jwt-key-rotation

[English](README.md) | [한국어](README.ko.md)

Distributed JWT signing-key example for `bluetape-go/jwt`, `jwt/redis`, and
`cache`.

This example models two API instances that share JWT signing keys through
Redis. One instance can force a `kid` rotation, while the other instance still
verifies tokens signed by retained keys. A trusted local reader cache keeps the
hot parse path fast without skipping distributed key revalidation.

## Scenario

`Service.IssueToken` signs an access token with the repository-backed current
HMAC key. `Service.RotateKey` forces a new key into Redis. `Service.VerifyToken`
uses `NewCachedDistributedProvider`, so repeated verification can reuse a local
reader only after the provider confirms the retained `kid` still exists.

Every public service operation creates a short child `context.Context` before
calling Redis-backed repository methods. Cancellation and deadlines are returned
instead of being converted into token errors.

## Architecture

- `jwt/redis.New` creates a caller-owned Redis key-chain repository.
- `jwt.NewDistributedHMACProvider` composes and parses JWTs with the shared
  repository.
- `cache.NewMemory[string,*jwt.Reader]` backs
  `jwt.NewCachedDistributedProvider` for repeated parse reads.
- The Gin router exposes token issue, profile verification, and explicit key
  rotation endpoints.

## Outcomes

| Case | Behavior | Test |
|---|---|---|
| Shared keys | Two services constructed with the same namespace verify each other's tokens. | `TestServiceSharesRotatedKeysAcrossInstances` |
| Rotation retention | A forced rotation changes the current `kid`, while the previous token remains valid inside the retained key window. | `TestServiceSharesRotatedKeysAcrossInstances` |
| Unknown or stale tokens | Tokens signed under another namespace and expired tokens map to stable public errors. | `TestServiceRejectsUnknownKIDAndExpiredToken` |
| Context budget | Cancelled contexts fail before Redis I/O can hide the caller's cancellation. | `TestServiceHonorsCancelledContextBeforeRepositoryIO` |
| Cached verification | Warm verification stays stable after rotation because cached readers revalidate the retained key. | `TestRouterUsesCachedProviderWithKeyRevalidation` |

## Run

Start Redis locally, then run the service:

```bash
export REDIS_ADDR=localhost:6379
go run ./examples/distributed-jwt-key-rotation
```

Issue and verify a token:

```bash
TOKEN=$(curl -s http://127.0.0.1:8099/tokens \
  -H 'Content-Type: application/json' \
  -d '{"subject":"customer-42","scopes":["orders:read"],"ttl_seconds":300}' |
  jq -r .access_token)

curl -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8099/profile
curl -X POST http://127.0.0.1:8099/keys/rotate
curl -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8099/profile
```

The tests start Redis with the repository Testcontainers fixture:

```bash
go test -count=1 ./examples/distributed-jwt-key-rotation/...
go test -race -count=1 ./examples/distributed-jwt-key-rotation/...
```
