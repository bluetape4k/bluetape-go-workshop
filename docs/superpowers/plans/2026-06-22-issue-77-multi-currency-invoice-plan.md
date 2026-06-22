# Multi-Currency Invoice Rule Evaluation Example Implementation Plan

> **For agentic workers:** Implement task-by-task. Keep checkbox state current
> when executing this plan in the same branch.

**Goal:** Add `examples/multi-currency-invoice-rules`, a focused Gin example
that evaluates multi-currency invoices with explicit money semantics and local
rule decisions.

**Architecture:** One example-local `internal/invoicerules` package owns
request validation, money parsing, per-currency accumulators, rule decisions,
public error mapping, and the Gin router. `main.go` only wires a loopback HTTP
server.

**Tech Stack:** Go, Gin, `github.com/bluetape4k/bluetape-go/money`, standard
library `errors`, `net/http`, `sort`, `strconv`, and `strings`. No new direct
dependencies.

## Constraints

- Use `bluetape-go/money` for all amount/currency handling.
- Keep rules example-local; do not add a generic rule engine.
- No exchange-rate conversion or cross-currency totals.
- Use tests before implementation.
- Keep docs bilingual and update root navigation.
- Preserve public error allowlist and avoid raw parser diagnostic leaks.

## Planned Files

- `examples/multi-currency-invoice-rules/main.go`
- `examples/multi-currency-invoice-rules/main_test.go`
- `examples/multi-currency-invoice-rules/README.md`
- `examples/multi-currency-invoice-rules/README.ko.md`
- `examples/multi-currency-invoice-rules/internal/invoicerules/service.go`
- `examples/multi-currency-invoice-rules/internal/invoicerules/service_test.go`
- `examples/multi-currency-invoice-rules/internal/invoicerules/http_test.go`
- `README.md`
- `README.ko.md`
- `docs/lessons/2026-06-22-multi-currency-invoice-rules.md`
- `docs/review/2026-06-22-issue-77-multi-currency-invoice-code-review.md`

## Implementation Tasks

- [x] **A. Service TDD red tests [complexity: medium]**
  - Add tests for grouped USD/EUR invoice totals, VIP service discount
    eligibility, EU tax-like adjustment, JPY zero-minor-unit rounding, and
    invalid currency rejection.
  - Run `go test -count=1 ./examples/multi-currency-invoice-rules/internal/invoicerules`
    and keep the expected compile/fail output as TDD evidence.

- [x] **B. Service implementation [complexity: medium]**
  - Implement DTOs, sentinels, `Service.Evaluate`, per-currency accumulators,
    `vip-service-discount`, `regional-vat`, money formatting, stable sorting,
    and public error mapping.
  - Run focused package tests.

- [x] **C. HTTP and main TDD/implementation [complexity: small]**
  - Add router tests for success, malformed JSON, invalid currency, and
    `/healthz`.
  - Add main tests for default loopback address, valid loopback override,
    non-loopback rejection, and bounded server timeouts.
  - Implement `main.go` with default `127.0.0.1:8100`.

- [x] **D. Documentation [complexity: medium]**
  - Add English/Korean example READMEs with scenario, endpoint table, run
    command, curl request, grouped totals response, invalid currency example,
    and production boundaries.
  - Update root English/Korean README tables and run sections.
  - Add lesson note linking #45 as the base example.

- [x] **E. Focused verification [complexity: medium]**
  - Run:
    - `go test -count=1 ./examples/multi-currency-invoice-rules/...`
    - `go test -race -count=1 ./examples/multi-currency-invoice-rules/...`
    - live smoke with `go run ./examples/multi-currency-invoice-rules` for
      `/healthz` and `/invoices/evaluate`.

- [x] **F. Full repository verification [complexity: high]**
  - Run:
    - `go test -p 1 ./...`
    - `make fmt-check`
    - `make tidy-check`
    - `make vet`
    - `make lint`
    - `GOFLAGS=-p=1 make ci`
    - `git diff --check`
  - Fix failures in scope.

- [x] **G. Step 6-R code review and fixes [complexity: medium]**
  - Run six-lane review plus security/trust-boundary review.
  - Save `docs/review/2026-06-22-issue-77-multi-currency-invoice-code-review.md`.
  - Fix all P0/P1 findings and rerun affected tests.

- [ ] **H. Commit, PR, merge, and local sync [complexity: small]**
  - Commit with Lore protocol.
  - Push branch and create a PR with `Closes #77`.
  - Match PR metadata from issue #77: assignee `debop`, milestone `0.6.0`,
    labels `enhancement` and `examples`.
  - End PR body with `## DoD Status`.
  - After CI passes, merge, sync local `develop`, and remove the worktree and
    local/remote feature branch.

## Acceptance Criteria Mapping

| Spec requirement | Plan coverage |
|---|---|
| Runnable example under `examples/` | A, B, C, D |
| Currency-specific rounding | A, B, E |
| Discount eligibility | A, B, E |
| Invalid currency rejection | A, B, C, E |
| README links #45 | D |
| Verification gates | E, F |
| Review and PR metadata | G, H |

## Risk Assumptions

- `regional-vat` is illustrative and should not be described as production tax
  compliance.
- There is no conversion policy by design; downstream examples can add
  settlement if needed.
- Rule decisions are intentionally plain structs, not a reusable framework.
