# Issue #44 ID/JWT Boundary 명세 리뷰

## 범위

- Branch: `feat/issue-44-id-jwt-boundary`
- Issue: #44 `[v0.6.0] Add ID and JWT boundary example`
- 검토 artifact:
  - `docs/superpowers/specs/2026-06-22-issue-44-id-jwt-boundary-design.md`

## Six-Lane 리뷰

| Lane | P0 | P1 | P2 | P3 | 결과 |
|---|---:|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | 0 | PASS |
| Stability | 0 | 0 | 0 | 0 | PASS |
| Security | 0 | 0 | 0 | 0 | PASS |
| Operator/Ops | 0 | 0 | 0 | 0 | PASS |
| Developer/API | 0 | 0 | 0 | 0 | PASS |
| User/Caller | 0 | 0 | 0 | 0 | PASS |

## finding

P0/P1 finding은 없다.

## 리뷰 note

- Performance: design에는 background worker, store, retry, sleep, external service가 없다.
  UUID/JWT operation은 request-local이며 bounded하다.
- Stability: service contract는 deterministic test를 위해 injected clock/generator/provider
  boundary를 사용하고 domain sentinel을 `errors.Is`로 매핑한다.
- Security: 선택된 scope는 auth framework 구축을 피하고 JWT claim이 encrypted가 아님을 명시적으로
  문서화하며 response에서 raw token, secret, parser diagnostic leak을 금지한다.
- Operator/Ops: public error code는 allowlisted되고 stable하다. live smoke command는 valid,
  missing, malformed, expired, forbidden path를 다룬다.
- Developer/API: example은 단일 `internal` package 아래에 머물고 새 helper abstraction 대신 기존
  `bluetape-go/id`와 `bluetape-go/jwt` API를 사용한다.
- User/Caller: 선택된 Gin route set은 README curl command에 충분히 작으면서도 #44가 요청한
  trust boundary를 보여준다.

## Acceptance 매핑

| Issue #44 criterion | spec coverage |
|---|---|
| Internal order ID and external request token flow | Scenario, HTTP Contract, Domain Contract |
| Generate IDs | ID Contract |
| Issue/verify JWT-like claims through upstream helpers | JWT Contract |
| Reject invalid tokens | HTTP Contract, Test Requirements |
| Avoid implying encoding is encryption | Goal, Non-Goals, Documentation Requirements |
| Tests for valid, expired, invalid token, and ID shape | Test Requirements |
| Secret handling and trust-boundary README guidance | Documentation Requirements |
| No real secrets committed | JWT Contract, Documentation Requirements |
| README pair in sync and root navigation update | Documentation Requirements |

## 수렴

spec은 implementation-ready 상태다.
