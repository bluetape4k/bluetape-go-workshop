# Token Refresh and Claims Validation Example Implementation Plan

> **For agentic workers:** Implement task-by-task. Keep the checkbox state
> current when executing this plan in the same branch.

**Goal:** Add `examples/token-refresh-claims`, a focused Gin example that
validates access-token claims and exchanges refresh tokens using
`github.com/bluetape4k/bluetape-go/jwt`.

**Architecture:** One example-local `internal/tokenrefresh` package owns token
issue, access validation, refresh validation, public error mapping, and the Gin
router. `main.go` only wires a loopback HTTP server. Tests drive the contract
with deterministic clock, secret, and ID generator.

**Tech Stack:** Go, Gin, `github.com/bluetape4k/bluetape-go/jwt`,
standard-library `errors`, `net/http`, `strings`, and `time`. No new direct
dependencies.

## Constraints

- Use `bluetape-go/jwt` helpers directly.
- Keep the example application-shaped; reusable auth/session code belongs
  elsewhere.
- Use tests before implementation.
- Keep public error responses allowlisted and secret-safe.
- Keep docs bilingual and update root navigation.
- Do not add databases, Redis, OIDC, JWKS, cookie sessions, revocation lists, or
  a generic auth middleware framework.

## Planned Files

- `examples/token-refresh-claims/main.go`
- `examples/token-refresh-claims/main_test.go`
- `examples/token-refresh-claims/README.md`
- `examples/token-refresh-claims/README.ko.md`
- `examples/token-refresh-claims/internal/tokenrefresh/service.go`
- `examples/token-refresh-claims/internal/tokenrefresh/service_test.go`
- `README.md`
- `README.ko.md`
- `docs/lessons/2026-06-22-token-refresh-claims.md`
- `docs/review/2026-06-22-issue-76-token-refresh-claims-code-review.md`

## Implementation Tasks

- [ ] **A. Service TDD red tests [complexity: medium]**
  - Add tests for session issue, valid access-token profile, expired access
    token, malformed/wrong-key token, wrong audience, wrong token-use, missing
    scope, valid refresh exchange, access-as-refresh rejection, and
    refresh-as-access rejection.
  - Assert public errors omit raw token, demo secret, and parser diagnostics.
  - Run `go test -count=1 ./examples/token-refresh-claims/internal/tokenrefresh`
    and keep the expected compile/fail output as TDD evidence.

- [ ] **B. Service implementation [complexity: medium]**
  - Implement sentinel errors, DTOs, `Service`, `IssueSession`,
    `ValidateAccess`, `RefreshAccess`, claim parsing helpers,
    bearer-token extraction, stable error mapping, and `NewRouter`.
  - Use `jwt.NewFixedHMACProvider(jwt.HS256, secret, jwt.WithClock(...),
    jwt.WithKeyIDGenerator(...))`.
  - Use deterministic injected ID generation in tests and a simple production
    entropy-backed generator for `session_id` and `jti`.
  - Run focused package tests.

- [ ] **C. Main entrypoint TDD/implementation [complexity: small]**
  - Add tests for default loopback address, valid loopback override,
    non-loopback rejection, and server timeouts.
  - Implement `main.go` with default `127.0.0.1:8097`, bounded HTTP server
    timeouts, signal handling, and graceful shutdown.
  - Run `go test -count=1 ./examples/token-refresh-claims/...`.

- [ ] **D. Documentation [complexity: medium]**
  - Add English/Korean example READMEs with scenario, endpoint table, run
    command, curl flow, valid profile, refresh, invalid token-use, and
    production hardening boundaries.
  - Update root `README.md` and `README.ko.md` example tables and run sections.
  - Link #44 as the base ID/JWT boundary example.
  - Add `docs/lessons/2026-06-22-token-refresh-claims.md`.

- [ ] **E. Focused verification [complexity: medium]**
  - Run:
    - `go test -count=1 ./examples/token-refresh-claims/...`
    - `go test -race -count=1 ./examples/token-refresh-claims/...`
    - live smoke with `go run ./examples/token-refresh-claims` for `/healthz`,
      `/sessions`, `/profile`, and `/tokens/refresh`.

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
  - Save `docs/review/2026-06-22-issue-76-token-refresh-claims-code-review.md`.
  - Fix all P0/P1 findings and rerun affected tests.

- [ ] **H. Commit and PR [complexity: small]**
  - Commit with Lore protocol.
  - Push branch and create a PR with `Closes #76`.
  - Match PR metadata from issue #76: assignee `debop`, milestone `0.6.0`,
    labels `enhancement` and `examples`.
  - End PR body with `## DoD Status`.

## Acceptance Criteria Mapping

| Spec requirement | Plan coverage |
|---|---|
| Runnable example | A, B, C, D |
| Valid claim validation | A, B, E |
| Expired token path | A, B, E |
| Invalid claims path | A, B, E |
| Refresh exchange behavior | A, B, D, E |
| README link to #44 | D |
| Verification gates | E, F |
| Review and PR metadata | G, H |

## Risk Assumptions

- This example is intentionally stateless and does not demonstrate refresh-token
  reuse detection. README must name durable session/revocation storage as
  production hardening.
- The fixed HMAC secret is intentionally committed as deterministic demo
  material. README must state that production systems must not copy it.
- Refresh and access tokens share signing material in the demo but use separate
  audiences and `token_use` claims. Production systems may use separate keys.
