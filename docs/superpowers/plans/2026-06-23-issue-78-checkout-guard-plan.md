# Issue #78 Implementation Plan

## Goal

Deliver `examples/checkout-guard-integration` with tests, bilingual docs, local
verification, PR, CI, merge, local sync, and worktree cleanup.

## Steps

1. Lock requirements from #78 and focused examples #44/#45/#46/#76/#77.
2. Add failing service and HTTP tests for authorized checkout, claim denial,
   rule denial, duplicate submission, money validation, concurrent admission,
   and loopback server config.
3. Implement `internal/checkoutguard`:
   - JWT demo token issue and access-token verification.
   - Checkout request validation and single-currency money pricing.
   - Local rule decisions for VIP discount, EU tax, and restricted category.
   - Bloom filter admission keyed by idempotency key.
   - Public error allowlist and Gin router.
4. Add `main.go` with loopback-only bind resolution and bounded HTTP timeouts.
5. Add English/Korean example READMEs, root README entries/run section, and
   lesson note.
6. Verify targeted tests, race test, full suite, lint/tidy/fmt/vet, and diff
   hygiene.
7. Code review the changed files, commit with Lore trailers, create PR with
   metadata matching #78, wait for CI, merge, sync `develop`, and remove the
   feature worktree.

## Step DoD

| Step | Done when |
|---|---|
| 1 | Spec and review capture #78 acceptance and non-goals. |
| 2 | Tests fail for missing `checkoutguard` implementation. |
| 3 | New example package passes targeted tests. |
| 4 | `main.go` tests pass and service can smoke-run locally. |
| 5 | Example and root docs are bilingual and consistent. |
| 6 | Required local verification commands pass or are explicitly documented. |
| 7 | PR is merged, local `develop` equals `origin/develop`, and no stale worktree remains. |
