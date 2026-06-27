# Distributed JWT key rotation example

Issue: #111

## Decision

Use `jwt/redis.New`, `jwt.NewDistributedHMACProvider`, and
`jwt.NewCachedDistributedProvider` directly in an application-shaped Gin example
instead of wrapping the distributed repository behind a fake store.

## Why

The lesson is not only JWT composition. The important boundary is that two API
instances share signing authority through Redis, force a new `kid`, and continue
to verify retained keys while local reader cache hits still revalidate the
repository key state.

## Verification shape

- Service tests start Redis through the repository Testcontainers fixture.
- Rotation tests assert both new `kid` issuance and old token retention.
- Cancellation tests keep caller `context.Context` errors visible before Redis
  I/O can be mistaken for token verification failures.
- Router tests exercise warm repeated verification through the cached
  distributed provider.
