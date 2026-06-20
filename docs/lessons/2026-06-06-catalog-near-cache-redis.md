# Catalog Near-Cache Redis Example Lessons

Issue: `#15 [v0.3.0] Add Redis near-cache and stampede coordination catalog example`

## What Worked

- A thin example-local `Peer` boundary was enough to demonstrate
  `cache.NewMemory`, `redisnear.NewPubSub`, and `rediscoord.NewStampedeCache`
  without creating reusable catalog infrastructure.
- `Store.LoadCount` made cache behavior observable. It proved both peer
  invalidation reloads and cross-peer cold-miss coordination.
- Replacing `bluetape-go/testing` with an example-local polling helper preserved
  the issue's no-new-dependencies acceptance criterion.
- Redis Testcontainers readiness needed a client `PING` after fixture startup.
  The fixture log wait alone was not always enough under race-test timing.

## Diagram Notes

- The README diagrams share English labels across `README.md` and
  `README.ko.md`.
- Each node-and-connector diagram keeps the final editable SVG and rendered PNG
  pair as the README asset source.
- `Architects Daughter` and `Comic Mono` are explicitly loaded in the final SVG
  assets with `@font-face` to avoid renderer font fallback.
- Visual inspection caught route/card overlap in early scenario and architecture
  renders; final SVGs were patched directly after layout review established the
  structure.
- Later visual review caught avoidable connector crossings in the split
  cold-miss and invalidation scenarios. Separate request and response corridors
  first, then reserve enough final straight segment before each card for the
  rendered arrowhead.

## Verification Notes

- `go test -count=1 ./examples/catalog-near-cache-redis/...`
- `go test -race -count=1 ./examples/catalog-near-cache-redis/...`
- `go test -count=10 -run TestColdMissBurstAcrossPeersRunsBackingLoaderOnce ./examples/catalog-near-cache-redis/...`
- `go test ./...`
- `golangci-lint cache clean && make ci`
- `git diff --check`

Initial `make ci` failed because golangci-lint cache referenced a deleted
sibling worktree. Cleaning the lint cache and rerunning the same command passed.

## Future Guidance

- For Redis-backed workshop examples, keep Testcontainers-backed commands
  serial and add a cheap client-level readiness check when the package opens
  Pub/Sub or lock connections immediately.
- If an example imports a bluetape-go test helper, run `go mod tidy` before
  committing and verify that no extra indirect requirements enter `go.mod`
  unless the issue explicitly permits them.
- For README SVG routes, run a simple H/V segment crossing sweep before
  accepting the PNG. If a line can avoid crossing by changing its port or
  corridor, fix the route instead of treating the crossing as acceptable.
