# Issue #77 설계: Multi-Currency Invoice Rule Evaluation 예제

## 프레임

- Issue: #77 `[v0.6.0] Add multi currency invoice rule evaluation example`
- Milestone: `0.6.0`
- 브랜치/워크트리: `feat/issue-77-multi-currency-invoice`, 위치는
  `.worktrees/feat-issue-77-multi-currency-invoice`.
- Base example: #45 `examples/money-rule-pricing`는 `money.Money`와 local rule
  decision으로 single-currency cart pricing을 설명한다.

## 목표

Currency별로 grouped invoice line을 평가하고, exchange-rate conversion 없이 명시적인
discount 및 tax-like rule을 적용하며, currency별 rounded total을 노출하는 실행
가능한 Gin 예제 `examples/multi-currency-invoice-rules`를 만든다.

이 예제는 invoice-level multi-currency semantics를 설명한다. General rule engine,
tax engine, exchange-rate service, accounting ledger, invoice persistence layer가
아니다.

## 근거

- `gh issue view 77`은 실행 가능한 예제, money utility, rule-engine style invoice
  decision, visible rounding, invalid currency rejection, #45를 base money/rule
  pricing 예제로 link하는 README navigation을 요구한다.
- `examples/money-rule-pricing`는 이미 `accepted`, `rejected`, `skipped` status로
  local rule decision을 증명한다.
- `github.com/bluetape4k/bluetape-go/money`는 명시적인 ISO 4217 currency,
  currency scale을 사용하는 `Money.Round()`, same-currency arithmetic을 제공한다.
  현재 module surface에는 dedicated reusable rules package가 없으므로 rule은
  example-local로 유지한다.

## HTTP 계약

| Method | Path | 목적 |
|---|---|---|
| `GET` | `/healthz` | Process liveness 전용. |
| `POST` | `/invoices/evaluate` | Invoice 하나를 평가하고 grouped currency total과 rule decision을 반환한다. |

## Request 계약

```json
{
  "invoice_id": "inv-1001",
  "customer_tier": "vip",
  "region": "EU",
  "lines": [
    {
      "line_id": "svc-1",
      "description": "Implementation workshop",
      "amount": "19.995",
      "currency": "USD",
      "quantity": 2,
      "category": "service"
    }
  ]
}
```

## Rule 계약

- `vip-service-discount`
  - `customer_tier = vip`일 때 service line에 5% discount를 적용한다.
  - Non-service line과 non-VIP customer는 명시적인 reason과 함께 skip한다.
- `regional-vat`
  - `region = EU`일 때 taxable line에 20% tax-like adjustment를 적용한다.
  - Line discount 이후의 line total에서 tax를 계산한다.
  - `tax_exempt` line과 non-EU region은 명시적인 reason과 함께 skip한다.
- Rule은 currency를 넘어 값을 결합하지 않는다. 각 rule decision은 currency와
  선택적인 rounded amount를 가진다.

## Money 계약

- 모든 line currency는 `money.ParseCurrency`로 parse한다.
- 모든 line amount는 `money.New`로 parse한다.
- 각 line total, discount, tax, final currency total은 `Money.Round()`로 round한다.
- Total은 currency code별로 group한다. Currency를 convert하거나 merge하지 않는다.
- `XXX`, empty, 그 밖의 invalid currency input은 `invalid_money`로 reject한다.

## Error 계약

Public response는 다음 형태를 사용한다.

```json
{"error_code":"invalid_money","message":"invalid money input"}
```

허용되는 public code:

- `invalid_request` (`400`)
- `invalid_money` (`400`)
- `invoice_error` (`500`)

Response는 raw invalid amount 또는 currency parser diagnostic을 echo하면 안 된다.

## 비목표

- Exchange rate 또는 cross-currency settlement 없음.
- Reusable rule framework 없음.
- Persistent invoice, payment capture, accounting export, locale tax compliance
  없음.
- 기존 Gin과 `bluetape-go/money`를 넘어서는 새 의존성 없음.

## 문서 요구사항

- `examples/multi-currency-invoice-rules` 아래에 English/Korean README pair를
  추가한다.
- Root English/Korean README table은 예제를 포함한다.
- Root run section은 #45를 base money/rule pricing 예제로 link하고, #77이 이를
  conversion 없는 multi-currency invoice로 확장한다고 설명한다.
- `docs/lessons/2026-06-22-multi-currency-invoice-rules.md`를 추가한다.

## 테스트 요구사항

- Zero-minor-unit currency를 포함해 currency-specific rounding이 보여야 한다.
- Discount eligibility는 VIP service line에만 적용된다.
- Invalid currency input은 `ErrInvalidMoney` 및 public `invalid_money`로
  reject된다.
- HTTP success 및 error path는 안정적인 public response shape로 mapping된다.
- `main.go`는 loopback binding과 bounded server timeout을 유지한다.

## 완료 기준

- Issue #77 acceptance criteria를 구현한다.
- PR body는 #77을 close하고 `## DoD Status`로 끝난다.
- PR metadata는 issue를 반영한다. Assignee `debop`, milestone `0.6.0`, label
  `enhancement`, `examples`.
