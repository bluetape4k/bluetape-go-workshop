# Money Rule Pricing Lesson 정리

Issue: #45

`examples/money-rule-pricing`는 `github.com/bluetape4k/bluetape-go/money`를 사용하는
cart pricing을 보여준다.

## 결정

모든 산술에는 `money.Money`를 사용하고 rule primitive는 예제 내부에 둔다. 현재
`bluetape-go` dependency는 money package를 제공하지만 general rule-engine package는
제공하지 않으므로, reusable rule abstraction은 workshop issue 범위를 벗어나는 framework
작업이 된다.

## 경계

- public monetary value는 string과 explicit currency로 표현한다.
- 산술을 시작하기 전에 item currency는 cart currency와 일치해야 한다.
- line total, discount total, final total은 cart boundary에서 round한다.
- cart가 그 외에는 valid하다면 rejected pricing rule은 `rules[]`에 보인다.
- invalid money나 mixed currency는 public HTTP error다.

## 후속 작업

Issue #77은 basic cart money arithmetic을 다시 가르치지 않고 explicit invoice-line
grouping과 conversion/no-conversion policy decision을 추가해 이 baseline 위에
multi-currency invoice rule evaluation을 만들 수 있다.
