# Issue #78 Design: Utility-Composed Checkout Guard Example

## Frame

- Issue: #78 `[v0.6.0] Add utility composed checkout guard integration example`
- Milestone: `0.6.0`
- Branch/worktree: `feat/issue-78-checkout-guard` under
  `.worktrees/feat-issue-78-checkout-guard`.
- Composed examples: #44 ID/JWT boundary, #76 token claim contracts, #45 money
  pricing, #46 probabilistic dedupe, and #77 money rule decisions.

## Goal

Create `examples/checkout-guard-integration`, a runnable Gin example that
combines first-party utility packages into one checkout submission boundary:
JWT claim verification, internal ID assignment, money/rule pricing, and Bloom
filter based repeated-submission protection.

This example teaches composition at a service boundary. It is not a payment
gateway, generic policy engine, authoritative idempotency store, distributed
checkout workflow, or reusable framework.

## Evidence

- `gh issue view 78` requires a Gin checkout API, deterministic in-memory
  state, composed lessons from #44/#45/#46/#76/#77, and tests for authorized
  checkout, denied claim/rule paths, duplicate submission, and money
  calculation behavior.
- `id-jwt-boundary` and `token-refresh-claims` already prove the JWT helper
  contract: explicit issuer/audience, `token_use`, role, scope, session ID, and
  public error allowlists.
- `money-rule-pricing` and `multi-currency-invoice-rules` prove decimal-backed
  `money.Money`, rounded totals, and local rule decisions without a reusable
  rules package.
- `probabilistic-dedupe-admission` proves the Bloom filter contract and the
  correct wording: `probably_seen` is a signal that can include false
  positives, not durable proof.

## HTTP Contract

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Process liveness only. |
| `POST` | `/tokens` | Issue a demo access token for local checkout requests. |
| `POST` | `/checkout/guard` | Verify token claims, price the checkout, evaluate rules, and admit or reject repeated submissions. |

The default bind address is `127.0.0.1:8101`.

## Request Contract

```json
{
  "checkout_id": "chk-1001",
  "idempotency_key": "idem-1001",
  "customer_tier": "vip",
  "region": "EU",
  "currency": "USD",
  "items": [
    {
      "line_id": "svc-1",
      "sku": "support-plan",
      "unit_price": "19.995",
      "currency": "USD",
      "quantity": 2,
      "category": "service"
    }
  ]
}
```

## Claim Contract

- Demo tokens are access tokens only: `token_use = access`.
- Checkout requires issuer `checkout-guard-integration`, audience
  `checkout-api`, role `customer`, and scope `checkout:submit`.
- Responses may expose subject and session ID after verification, but never raw
  parser diagnostics or token internals.

## Rule And Money Contract

- The checkout request has one settlement currency. Every line must use that
  same currency.
- Use `money.ParseCurrency`, `money.New`, same-currency arithmetic, and
  `Money.Round()` for line totals, discounts, taxes, and final totals.
- `vip-service-discount` applies a 5% discount to VIP service lines.
- `regional-vat` applies a 20% tax-like adjustment to EU taxable lines after
  discount.
- `restricted-category` rejects items in category `restricted` as a local
  eligibility rule.

## Dedupe Contract

- Use an in-memory Bloom filter keyed by `idempotency_key`.
- First unseen key returns `admit` / `definitely_new`.
- Repeated keys return `probably_seen` / `might_be_duplicate_or_false_positive`
  and the HTTP boundary maps that to `409 duplicate_submission`.
- Documentation must state that production checkout idempotency needs a durable
  authoritative store in addition to the probabilistic signal.

## Error Contract

Public responses use this shape:

```json
{"error_code":"duplicate_submission","message":"The checkout was already seen or may be a false positive."}
```

Allowed public codes:

- `missing_token` (`401`)
- `expired_token` (`401`)
- `invalid_token` (`401`)
- `invalid_claims` (`403`)
- `invalid_request` (`400`)
- `invalid_money` (`400`)
- `rule_denied` (`422`)
- `duplicate_submission` (`409`)
- `checkout_error` (`500`)

## Non-Goals

- No reusable rule framework.
- No payment capture, fulfillment workflow, inventory reservation, ledger, or
  persistent checkout store.
- No token refresh flow; #76 owns that focused example.
- No multi-currency conversion; #77 owns grouped multi-currency invoice totals.
- No new dependencies beyond existing Gin and `bluetape-go` packages.

## Documentation Requirements

- Add English/Korean README pair under `examples/checkout-guard-integration`.
- Root English/Korean README tables include the example.
- Root run sections include token issue and guarded checkout commands.
- Add `docs/lessons/2026-06-23-checkout-guard-integration.md`.

## Test Requirements

- Authorized checkout returns request/order IDs, verified subject/session,
  `admit`, rule decisions, and rounded money totals.
- Valid but under-scoped or wrong-role tokens are denied as `ErrInvalidClaims`
  and public `invalid_claims`.
- Restricted category requests are denied as `ErrRuleDenied`.
- Reusing an idempotency key is denied as `ErrDuplicateSubmission` and public
  `duplicate_submission`.
- Invalid or mismatched money input is denied as `ErrInvalidMoney`.
- Concurrent duplicate submissions admit exactly one request.
- `main.go` keeps loopback binding and bounded server timeouts.

## Completion Criteria

- Issue #78 acceptance criteria are implemented.
- `go test -race` passes for the new example package.
- PR body closes #78 and ends with `## DoD Status`.
- PR metadata mirrors the issue: assignee `debop`, milestone `0.6.0`, labels
  `enhancement` and `examples`.
