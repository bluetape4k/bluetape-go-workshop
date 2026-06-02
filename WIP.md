# WIP

Snapshot: 2026-06-02 KST
Scope: open GitHub issues assigned to `debop`.
Open count: 0 issues.

## Current Milestone

Bootstrap a thin workshop repository that demonstrates `bluetape-go` packages in
real web application shapes.

## Current Scope

- Keep examples small and runnable.
- Use `bluetape-go` packages directly instead of duplicating library logic.
- Use `chi` as the default lightweight web framework when examples need routing
  or middleware.
- Run Testcontainers-backed tests in local CI, GitHub CI, and Nightly.

## Next Examples

- Add resilience examples after `bluetape-go` provides the first resilience package.
- Add near-cache examples after cache coordination packages exist.
- Add state, workflow, and batch examples when their APIs stabilize.

## Decision Log

- Keep application examples in `bluetape-go-workshop`, not in `bluetape-go`.
- Start with one Redis leader web example so the workshop does not outrun the library.
- Prefer lightweight `chi` examples over full-stack framework abstractions.
- Use `-count=1` in test commands so Go's test cache cannot hide Testcontainers execution.
