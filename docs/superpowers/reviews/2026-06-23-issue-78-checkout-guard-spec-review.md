# Issue #78 명세 리뷰

## 판정

implementation을 진행한다. scope는 각 package의 lesson을 좁게 유지하고 새 checkout framework를
피하면서도 올바르게 integration-shaped다.

## Six-Lane 리뷰

| Lane | 관심사 | 결정 |
|---|---|---|
| Correctness | money arithmetic | checkout마다 하나의 settlement currency를 유지하고 mismatched line currency를 거부한다. |
| Domain | dedupe semantic | Bloom output 이름을 `probably_seen`으로 지정하고 durable idempotency를 production requirement로 문서화한다. |
| Security | JWT boundary | checkout response 전에 `token_use`, issuer, audience, role, scope, session ID를 요구한다. |
| Stability | shared state | concurrent duplicate test를 위해 Bloom admission과 deterministic test ID를 guard한다. |
| Developer/API | reuse | first-party `id`, `jwt`, `money`, `probabilistic` API를 직접 사용하고 sibling `internal` example은 import하지 않는다. |
| User/Docs | composition story | README는 broad framework를 도입하지 않고 focused example이 조합되는 방식을 보여야 한다. |

## 필요한 조정

- route가 짧고 integration-focused임이 명확하도록 example을
  `examples/checkout-guard-integration`에 둔다.
- duplicate submission에는 `409`를 반환하되 Bloom match가 false positive일 수 있음을 설명한다.
- public error mapping에 대한 service-level test와 HTTP-level test를 모두 추가한다.
- example이 shared admission state를 소유하므로 새 package에 race run을 포함한다.

## blocker

None.
