# Shared Redis Bloom admission example

Issue: #113

## Decision

Use `probabilistic/redis.NewStringBloomFilter` in a runnable Gin webhook
admission example, with real Redis-backed tests through the repository
Testcontainers fixture.

## Why

The in-memory Bloom admission example shows the local `definitely_new` versus
`probably_seen` boundary, but it cannot show process-to-process visibility. This
example moves the Bloom bits and metadata to Redis so two application instances
using the same namespace see the same probabilistic state.

The app owns the HTTP request contract, stable public decisions, stable error
mapping, and reader caveats. `bluetape-go/probabilistic/redis` owns Bloom sizing,
Redis storage, metadata fingerprint checks, bit operations, and approximate
stats.

## Verification shape

- Service tests assert first insert, repeated value, cross-instance visibility,
  config mismatch, Redis failure, cancellation, and invalid request behavior.
- Router tests assert stable `admit`, `probably_seen`, stats, health, and public
  invalid request mappings.
- README diagrams show the shared Redis architecture, cross-instance sequence,
  and decision policy separately so readers can see why Bloom hits are
  prefilter signals rather than authorization or exact dedupe.
- Docker-backed tests run serially with `-p 1` to avoid shared container
  contention.
