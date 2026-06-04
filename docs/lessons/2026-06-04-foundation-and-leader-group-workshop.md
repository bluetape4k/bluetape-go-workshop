# Foundation And Leader Group Workshop Examples

## Context

`bluetape-go-workshop` still had open 0.1.0 foundation example issues and one
0.2.0 LeaderGroupElector example issue after the resilience HTTP example landed.

## Decision

Add scenario-first examples instead of API catalog pages. Keep each package thin
and prove snippets with normal `go test`, Testcontainers integration tests, and
stress helpers where concurrency behavior matters.

## Outcome

The workshop now covers cache snapshot codecs, order feed cleanup, invitation
codecs, Redis leader jobs, product enrichment fan-out, Testcontainers-backed
order pipeline integration, and Redis leader group web coordination.

## Verification

- `go test -count=1 ./examples/cache-snapshot-codecs/... ./examples/order-intake-cleanup/... ./examples/invitation-codecs/... ./examples/product-enrichment-fanout/...`
- `go test -count=1 ./examples/leader-coordination-jobs/... ./examples/order-pipeline-testcontainers/... ./examples/leader-group-web/...`
- `go test -count=1 ./...`
- `make ci`
- `git diff --check`

## Future Guard

For Go feature examples with goroutine, cancellation, async, shared-state, or
coordination behavior, include `GoroutineStressTester` or `AsyncJobTester`
coverage before opening the PR.
