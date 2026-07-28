# Issue #77 명세 리뷰

## 판정

example을 local하고 narrow하게 유지한 뒤 implementation을 진행한다. 새 generic rule engine을
피하면서 `money`를 rule-style decision과 조합하므로 scope는 #77과 일관된다.

## Six-Lane 리뷰

| Lane | 관심사 | 결정 |
|---|---|---|
| Correctness | multi-currency arithmetic | currency별로 group하고 currency boundary를 넘는 add/subtract를 하지 않는다. |
| Domain | tax-like rule | `regional-vat`는 compliance language가 아니라 illustrative example로 명확히 유지한다. |
| Security | HTTP boundary | JSON body를 cap하고 trusted proxy를 disable하며 error에서 raw parser diagnostic을 피한다. |
| Stability | determinism | stable test와 docs를 위해 currency total과 rule decision을 sort한다. |
| Developer/API | reuse | `money.Money`를 reuse한다. reusable rule package가 없으므로 rule은 example-local로 유지한다. |
| User/Docs | navigation | #45를 base single-currency money/rule pricing lesson으로 link한다. |

## 필요한 조정

- issue scope가 path와 README table에 보이도록 example 이름을 `multi-currency-invoice-rules`로
  지정한다.
- USD/EUR 밖의 currency scale이 보이도록 zero-minor-unit rounding test 하나를 포함한다.
  가능하면 JPY를 사용한다.
- 모든 request amount field를 string으로 유지하고 `float64` arithmetic을 피한다.
- exchange-rate conversion이 발생하지 않는다고 문서화한다.

## blocker

None.
