# Probabilistic Dedupe Admission Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a runnable Gin example that uses `bluetape-go/probabilistic` to prefilter event IDs as definitely new or probably seen.

**Architecture:** The example has a small `dedupe.Service` that owns one `probabilistic.BloomFilter[string]`, returns stable response DTOs, and exposes a Gin router. The Bloom filter is an admission prefilter only; docs explain that production dedupe still needs durable authoritative storage.

**Tech Stack:** Go, Gin, `github.com/bluetape4k/bluetape-go/probabilistic`, standard `net/http` server timeouts.

---

## File Structure

- Create `examples/probabilistic-dedupe-admission/main.go`: loopback-only runnable server on `127.0.0.1:8099`.
- Create `examples/probabilistic-dedupe-admission/internal/dedupe/service.go`: service, DTOs, router, error mapping, and stats projection.
- Create `examples/probabilistic-dedupe-admission/internal/dedupe/service_test.go`: service-level tests for admit/probably-seen/invalid/stats behavior.
- Create `examples/probabilistic-dedupe-admission/internal/dedupe/http_test.go`: router tests for HTTP success, repeat event, public errors, and health.
- Create `examples/probabilistic-dedupe-admission/README.md` and `README.ko.md`: run instructions, sample curl calls, false-positive and durable-store notes.
- Modify root `README.md` and `README.ko.md`: examples table and run section.
- Create `docs/lessons/2026-06-22-probabilistic-dedupe-admission.md`: decision record and follow-up context.
- Create `docs/review/2026-06-22-issue-46-probabilistic-dedupe-code-review.md`: Step 6-R review artifact after implementation.

## Tasks

### Task 1: Service TDD

- [ ] Write failing tests in `examples/probabilistic-dedupe-admission/internal/dedupe/service_test.go`:
  - `TestServiceAdmitsDefinitelyNewEvent`
  - `TestServiceMarksRepeatedEventAsProbablySeen`
  - `TestServiceRejectsInvalidEventID`
  - `TestServiceStatsReflectAdmissions`
- [ ] Run `go test -count=1 ./examples/probabilistic-dedupe-admission/...` and verify failure from undefined service/types.
- [ ] Implement `Service`, `NewService`, `Admit`, `Stats`, DTOs, and sentinel errors.
- [ ] Run `go test -count=1 ./examples/probabilistic-dedupe-admission/...` and verify service tests pass.

### Task 2: HTTP TDD

- [ ] Write failing tests in `examples/probabilistic-dedupe-admission/internal/dedupe/http_test.go`:
  - `TestRouterAdmitsAndThenMarksProbablySeen`
  - `TestRouterMapsInvalidRequest`
  - `TestHealthz`
- [ ] Run targeted tests and verify router symbols are missing or failing.
- [ ] Implement `NewRouter`, `/healthz`, `/events/admit`, `/filters/current`, max JSON body, and public error responses.
- [ ] Run targeted tests and verify pass.

### Task 3: Runnable Example and Docs

- [ ] Add `main.go` with loopback-only `HTTP_ADDR`, server timeouts, signal shutdown, and default port `8099`.
- [ ] Add English/Korean example READMEs with run commands, first/repeat event curl calls, stats endpoint, false-positive notes, and durable-store pairing.
- [ ] Update root English/Korean README examples table and run section.
- [ ] Add lesson note under `docs/lessons`.

### Task 4: Verification and Review

- [ ] Run `gofmt` on changed Go files.
- [ ] Run `go test -count=1 ./examples/probabilistic-dedupe-admission/...`.
- [ ] Run `go test -race -count=1 ./examples/probabilistic-dedupe-admission/...`.
- [ ] Run `go test -p 1 ./...`.
- [ ] Run `make ci`.
- [ ] Run `git diff --check`.
- [ ] Run Step 6-R local six-lane review, fix P0/P1, and store review artifact under `docs/review`.

### Task 5: Commit and PR

- [ ] Commit with Lore trailers and validation evidence.
- [ ] Push `feat/issue-46-probabilistic-dedupe`.
- [ ] Create PR against `develop`, linked with `Closes #46`, assignee `debop`, labels `enhancement` and `examples`, milestone `0.6.0`.
- [ ] Verify PR body ends with `## DoD Status`.
- [ ] Wait for GitHub CI.
- [ ] If CI passes, rebase merge on the user-approved current continuation path, sync local `develop`, remove worktree, and delete local/remote feature branch.

## Step 3-R Integrated Review

Native subagent spawning is not available in this Codex surface, so the six review lanes were run as independent main-session checks using the full-feature reference contract.

| Priority | Area | Finding | Required plan edit |
|---|---|---|---|
| P2 | Stability | Shared filter state needs a changed-package race test. | Add targeted race task. Done. |
| P2 | User | README must not imply the Bloom filter is authoritative. | Add false-positive and durable-store doc task. Done. |
| P2 | Developer | HTTP tests must cover repeat event, not just service tests. | Add router duplicate-path task. Done. |
| P3 | Operator | Stats endpoint should expose approximate values without production telemetry claims. | Add stats projection only. Done. |

Final Step 3-R verdict: P0 = 0, P1 = 0.
