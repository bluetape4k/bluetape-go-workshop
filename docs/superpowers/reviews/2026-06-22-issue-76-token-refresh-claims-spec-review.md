# Issue #76 Token Refresh Claims 명세 리뷰

## 범위

- Branch: `feat/issue-76-token-refresh`
- Issue: #76 `[v0.6.0] Add token refresh and claims validation example`
- 검토 artifact:
  - `docs/superpowers/specs/2026-06-22-issue-76-token-refresh-claims-design.md`

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

- Performance: JWT parsing/signing은 request-local이며 background worker, sleep, store,
  network service를 도입하지 않는다.
- Stability: deterministic clock과 ID injection은 expiration, `jti`, `session_id` assertion을
  reproducible하게 만든다.
- Security: spec은 access-token과 refresh-token의 `aud`와 `token_use`를 분리하고,
  allowlisted public error contract를 요구하며 durable session-store claim을 명시적으로 피한다.
- Operator/Ops: `main.go`는 #44의 loopback-only bind와 bounded server timeout pattern을
  상속하므로 unauthenticated demo는 기본적으로 노출되지 않는다.
- Developer/API: example은 기존 `bluetape-go/jwt` API를 직접 사용하고 모든 app policy를
  `internal` package에 local로 유지한다.
- User/Caller: route set은 README curl flow에 충분히 작으면서 valid access, expired access,
  invalid claim, refresh exchange를 다룬다.

## Acceptance 매핑

| Issue #76 criterion | spec coverage |
|---|---|
| Runnable example under `examples/` | Goal, HTTP Contract |
| Valid claims tests | Test Requirements |
| Expired token tests | Test Requirements |
| Invalid claims tests | JWT Contract, Test Requirements |
| Refresh behavior tests | HTTP Contract, JWT Contract, Test Requirements |
| README links #44 base example | Documentation Requirements |

## 수렴

spec은 implementation-ready 상태다.
