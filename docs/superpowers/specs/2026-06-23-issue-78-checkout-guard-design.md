# Issue #78 설계: Utility-Composed Checkout Guard 예제

## 프레임

- Issue: #78 `[v0.6.0] Add utility composed checkout guard integration example`
- Milestone: `0.6.0`
- 브랜치/워크트리: `feat/issue-78-checkout-guard`, 위치는
  `.worktrees/feat-issue-78-checkout-guard`.
- Composed example: #44 ID/JWT boundary, #76 token claim contract, #45 money
  pricing, #46 probabilistic dedupe, #77 money rule decision.

## 목표

JWT claim verification, internal ID assignment, money/rule pricing, Bloom
filter 기반 repeated-submission protection을 하나의 checkout submission boundary로
결합하는 실행 가능한 Gin 예제 `examples/checkout-guard-integration`을 만든다.

이 예제는 service boundary에서의 composition을 설명한다. Payment gateway, generic
policy engine, authoritative idempotency store, distributed checkout workflow,
reusable framework가 아니다.

## 근거

- `gh issue view 78`은 Gin checkout API, deterministic in-memory state,
  #44/#45/#46/#76/#77의 composed lesson, authorized checkout, denied claim/rule
  path, duplicate submission, money calculation behavior 테스트를 요구한다.
- `id-jwt-boundary`와 `token-refresh-claims`는 explicit issuer/audience,
  `token_use`, role, scope, session ID, public error allowlist라는 JWT helper
  contract를 이미 증명한다.
- `money-rule-pricing`와 `multi-currency-invoice-rules`는 reusable rules package
  없이 decimal-backed `money.Money`, rounded total, local rule decision을
  증명한다.
- `probabilistic-dedupe-admission`은 Bloom filter contract와 올바른 표현을
  증명한다. `probably_seen`은 false positive를 포함할 수 있는 signal이며 durable
  proof가 아니다.

## HTTP 계약

| Method | Path | 목적 |
|---|---|---|
| `GET` | `/healthz` | Process liveness 전용. |
| `POST` | `/tokens` | Local checkout request용 demo access token을 발급한다. |
| `POST` | `/checkout/guard` | Token claim을 검증하고 checkout 가격을 계산하며 rule을 평가하고 repeated submission을 admit 또는 reject한다. |

기본 bind address는 `127.0.0.1:8101`이다.

## Request 계약

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

## Claim 계약

- Demo token은 access token 전용이다. `token_use = access`.
- Checkout은 issuer `checkout-guard-integration`, audience `checkout-api`, role
  `customer`, scope `checkout:submit`을 요구한다.
- Response는 검증 이후 subject와 session ID를 노출할 수 있지만, raw parser
  diagnostic이나 token internal은 절대 노출하지 않는다.

## Rule 및 Money 계약

- Checkout request는 하나의 settlement currency를 가진다. 모든 line은 같은
  currency를 사용해야 한다.
- Line total, discount, tax, final total에는 `money.ParseCurrency`, `money.New`,
  same-currency arithmetic, `Money.Round()`를 사용한다.
- `vip-service-discount`는 VIP service line에 5% discount를 적용한다.
- `regional-vat`는 discount 이후 EU taxable line에 20% tax-like adjustment를
  적용한다.
- `restricted-category`는 category `restricted` item을 local eligibility rule로
  reject한다.

## Dedupe 계약

- `idempotency_key`를 key로 사용하는 in-memory Bloom filter를 사용한다.
- 처음 보는 key는 `admit` / `definitely_new`를 반환한다.
- 반복된 key는 `probably_seen` / `might_be_duplicate_or_false_positive`를
  반환하고, HTTP boundary는 이를 `409 duplicate_submission`으로 mapping한다.
- 문서는 production checkout idempotency에 probabilistic signal 외에도 durable
  authoritative store가 필요하다고 설명해야 한다.

## Error 계약

Public response는 다음 형태를 사용한다.

```json
{"error_code":"duplicate_submission","message":"The checkout was already seen or may be a false positive."}
```

허용되는 public code:

- `missing_token` (`401`)
- `expired_token` (`401`)
- `invalid_token` (`401`)
- `invalid_claims` (`403`)
- `invalid_request` (`400`)
- `invalid_money` (`400`)
- `rule_denied` (`422`)
- `duplicate_submission` (`409`)
- `checkout_error` (`500`)

## 비목표

- Reusable rule framework 없음.
- Payment capture, fulfillment workflow, inventory reservation, ledger,
  persistent checkout store 없음.
- Token refresh flow 없음. #76이 해당 focused example을 소유한다.
- Multi-currency conversion 없음. #77이 grouped multi-currency invoice total을
  소유한다.
- 기존 Gin과 `bluetape-go` package를 넘어서는 새 의존성 없음.

## 문서 요구사항

- `examples/checkout-guard-integration` 아래에 English/Korean README pair를
  추가한다.
- Root English/Korean README table은 예제를 포함한다.
- Root run section은 token issue 및 guarded checkout command를 포함한다.
- `docs/lessons/2026-06-23-checkout-guard-integration.md`를 추가한다.

## 테스트 요구사항

- Authorized checkout은 request/order ID, verified subject/session, `admit`, rule
  decision, rounded money total을 반환한다.
- Valid하지만 under-scoped 또는 wrong-role token은 `ErrInvalidClaims` 및 public
  `invalid_claims`로 deny된다.
- Restricted category request는 `ErrRuleDenied`로 deny된다.
- Idempotency key 재사용은 `ErrDuplicateSubmission` 및 public
  `duplicate_submission`으로 deny된다.
- Invalid 또는 mismatched money input은 `ErrInvalidMoney`로 deny된다.
- Concurrent duplicate submission은 정확히 하나의 request만 admit한다.
- `main.go`는 loopback binding과 bounded server timeout을 유지한다.

## 완료 기준

- Issue #78 acceptance criteria를 구현한다.
- 새 example package에서 `go test -race`가 통과한다.
- PR body는 #78을 close하고 `## DoD Status`로 끝난다.
- PR metadata는 issue를 반영한다. Assignee `debop`, milestone `0.6.0`, label
  `enhancement`, `examples`.
