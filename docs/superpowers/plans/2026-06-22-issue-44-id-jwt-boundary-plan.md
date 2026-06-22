# ID and JWT Boundary Example Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `examples/id-jwt-boundary`, a runnable Gin example that combines
`bluetape-go/id` UUID v7 identifiers and `bluetape-go/jwt` fixed-HMAC demo
tokens while teaching the boundary between identifiers, signed claims, and
authorization policy.

**Architecture:** Keep one example-local `internal/idjwtboundary` package. The
service owns token issuance, token validation, role/scope checks, order request
validation, UUID v7 generation, and stable public error mapping. `main.go` only
wires a loopback HTTP server. Tests drive the contract first with deterministic
clock/token/ID dependencies.

**Tech Stack:** Go, Gin, `github.com/bluetape4k/bluetape-go/id`,
`github.com/bluetape4k/bluetape-go/jwt`, standard-library `errors`,
`net/http`, `strings`, `sync`, and `time`. No new direct dependencies.

---

## Constraints

- Apply `$bluetape-go-patterns`: context-aware boundaries where relevant,
  sentinel errors, table-driven tests, race-safe dependencies, and `gofmt`.
- Keep the example application-shaped; reusable library code belongs in
  `bluetape-go`.
- Use tests before implementation.
- Keep public error responses allowlisted and secret-safe.
- Keep docs bilingual and update root navigation.
- Do not add databases, Redis, Testcontainers, sessions, OIDC, JWKS, or a
  generic auth middleware framework.

## Planned Files

- `examples/id-jwt-boundary/main.go`
- `examples/id-jwt-boundary/main_test.go`
- `examples/id-jwt-boundary/README.md`
- `examples/id-jwt-boundary/README.ko.md`
- `examples/id-jwt-boundary/internal/idjwtboundary/service.go`
- `examples/id-jwt-boundary/internal/idjwtboundary/service_test.go`
- `README.md`
- `README.ko.md`
- `docs/lessons/2026-06-22-id-jwt-boundary.md`
- `docs/review/2026-06-22-issue-44-id-jwt-boundary-code-review.md`

## Implementation Tasks

- [ ] **A. Service TDD red tests [complexity: medium]**
  - Create tests for valid token order creation, missing token, expired token,
    malformed token, wrong-key token, forbidden scope, invalid JSON/request
    validation, UUID v7 shape, and ID generator failure.
  - Assert public errors omit raw token, demo secret, and raw JWT parser text.
  - Run `go test -count=1 ./examples/id-jwt-boundary/internal/idjwtboundary`
    and keep the expected compile/fail output as TDD evidence.

- [ ] **B. Service implementation [complexity: medium]**
  - Implement sentinel errors, DTOs, `Service`, `IssueToken`, `CreateOrder`,
    claim parsing helpers, bearer-token extraction, stable error mapping, and
    `NewRouter`.
  - Use `jwt.NewFixedHMACProvider(jwt.HS256, secret, jwt.WithClock(...),
    jwt.WithKeyIDGenerator(...))`.
  - Use `id.NewUUIDV7Generator` in production wiring and an injected generator
    in tests.
  - Run focused package tests.

- [ ] **C. Main entrypoint TDD/implementation [complexity: small]**
  - Add tests for default loopback address, `HTTP_ADDR` override, non-loopback
    rejection, and server timeouts.
  - Implement `main.go` with default `127.0.0.1:8096`, bounded HTTP server
    timeouts, signal handling, and graceful shutdown.
  - Run `go test -count=1 ./examples/id-jwt-boundary/...`.

- [ ] **D. Documentation [complexity: medium]**
  - Add English/Korean example READMEs with scenario, endpoint table, run
    command, curl flow, valid/missing/expired/invalid/forbidden examples, and
    production hardening boundaries.
  - Update root `README.md` and `README.ko.md` example tables and run sections.
  - Add `docs/lessons/2026-06-22-id-jwt-boundary.md`.

- [ ] **E. Focused verification [complexity: medium]**
  - Run:
    - `go test -count=1 ./examples/id-jwt-boundary/...`
    - `go test -race -count=1 ./examples/id-jwt-boundary/...`
    - live smoke with `go run ./examples/id-jwt-boundary` for `/healthz`,
      token issue, valid order, missing token, expired token, malformed token,
      and forbidden scope.

- [ ] **F. Full repository verification [complexity: high]**
  - Run:
    - `go test -p 1 ./...`
    - `make fmt-check`
    - `make tidy-check`
    - `make vet`
    - `make lint`
    - `GOFLAGS=-p=1 make ci`
    - `git diff --check`
  - Fix failures in scope.

- [ ] **G. Step 6-R code review and fixes [complexity: medium]**
  - Run six-lane review plus security/trust-boundary review.
  - Save `docs/review/2026-06-22-issue-44-id-jwt-boundary-code-review.md`.
  - Fix all P0/P1 findings and rerun affected tests.

- [ ] **H. Commit and PR [complexity: small]**
  - Commit with Lore protocol.
  - Push branch and create a PR with `Closes #44`.
  - Match PR metadata from issue #44: assignee `debop`, milestone `0.6.0`,
    labels `enhancement` and `examples`.
  - End PR body with `## DoD Status`.

## Acceptance Criteria Mapping

| Spec requirement | Plan coverage |
|---|---|
| Internal order ID and request token flow | A, B, D, E |
| JWT issue/verify and invalid rejection | A, B, D, E |
| Expired token path | A, B, D, E |
| ID shape tests | A, B |
| Secret handling and trust-boundary docs | D, G |
| Root navigation | D |
| Verification gates | E, F |
| Review and PR metadata | G, H |

## Risk Assumptions

- `go mod tidy` may add checksums for transitive `bluetape-go/id` and
  `bluetape-go/jwt` dependencies that are not currently used by the workshop
  module.
- The fixed HMAC secret is intentionally committed as deterministic demo
  material. README must explicitly state that production systems must not copy
  it.
- The example is not a complete authorization system; role/scope checks are
  local policy hooks for the boundary lesson only.
