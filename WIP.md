# WIP

Snapshot: 2026-06-04 KST
Scope: open GitHub issues assigned to `debop`.
Open count: 7 issues.

## Current Milestone

Bootstrap a thin workshop repository that demonstrates `bluetape-go` packages in
real web application shapes.

## Current Scope

- Keep examples small and runnable.
- Use `bluetape-go` packages directly instead of duplicating library logic.
- Use `chi` as the default lightweight web framework when examples need routing
  or middleware.
- Run Testcontainers-backed tests in local CI, GitHub CI, and Nightly.
- Finish `0.1.0` foundation examples and `0.2.0` leader group examples before
  closing their workshop milestones.

## Next Examples

- Add near-cache examples after cache coordination packages exist.
- Add state, workflow, and batch examples when their APIs stabilize.

## Decision Log

- Keep application examples in `bluetape-go-workshop`, not in `bluetape-go`.
- Start with one Redis leader web example so the workshop does not outrun the library.
- Prefer lightweight `chi` examples over full-stack framework abstractions.
- Use `-count=1` in test commands so Go's test cache cannot hide Testcontainers execution.
- For Go feature examples, include stress validation with `GoroutineStressTester`
  and `AsyncJobTester` when concurrency, goroutine, async, cancellation, or
  shared-state behavior is involved.
- Keep examples scenario-first: each example should solve one business-shaped
  problem rather than cataloging every helper function.
