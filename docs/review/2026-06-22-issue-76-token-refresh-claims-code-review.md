# Issue #76 Token Refresh Claims Code Review

## 범위

- Branch: `feat/issue-76-token-refresh`
- Issue: #76 `[v0.6.0] Add token refresh and claims validation example`
- Planning commit: `0000001 Define the token refresh claims boundary`
- 검토 파일:
  - `examples/token-refresh-claims/**`
  - `README.md`
  - `README.ko.md`
  - `docs/lessons/2026-06-22-token-refresh-claims.md`

## 발견 사항

P0/P1 finding은 없다.

## Acceptance Evidence

- runnable example: `examples/token-refresh-claims/main.go`가 `/healthz`, `/sessions`,
  `/profile`, `/tokens/refresh`를 가진 loopback Gin API를 wiring한다.
- valid claims: `/profile`은 issuer, access audience, `token_use=access`, role, scope,
  subject, `session_id`가 있는 access token을 accept한다.
- expired tokens: test는 expired access/refresh token을 구성하고 `expired_token`을 assert한다.
- invalid claims: test는 wrong audience, wrong `token_use`, missing scope,
  refresh-as-access, access-as-refresh를 다룬다.
- refresh behavior: valid refresh token은 `/profile`이 accept하는 새 access token을 반환한다.
- README linkage: English/Korean root 및 example README는 #44 `examples/id-jwt-boundary`를
  base boundary lesson으로 link한다.

## Six-Lane Review

| Lane | P0 | P1 | P2 | P3 | Result |
|---|---:|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | 0 | PASS |
| Stability | 0 | 0 | 0 | 0 | PASS |
| Security | 0 | 0 | 0 | 0 | PASS |
| Operator/Ops | 0 | 0 | 0 | 0 | PASS |
| Developer/API | 0 | 0 | 0 | 0 | PASS |
| User/Caller | 0 | 0 | 0 | 0 | PASS |

## Review 메모

- Performance: token issue/parse work는 request-local이다. background worker, store,
  network client, sleep, retry loop를 추가하지 않는다.
- Stability: test는 clock, secret, deterministic ID generation을 inject한다. HTTP body size는
  JSON binding 전에 8 KiB로 제한된다.
- Security: access token과 refresh token은 audience 및 `token_use`로 분리된다. public error
  response는 allowlisted이고 test는 raw token, demo secret, parser diagnostic leak이 없음을
  assert한다.
- Operator/Ops: entrypoint는 non-loopback `HTTP_ADDR` 값을 거부하고 bounded read-header,
  read, write, idle timeout을 사용한다.
- Developer/API: 예제는 `internal/tokenrefresh` package에 머물며 기존 `bluetape-go/jwt` API를
  직접 사용한다.
- User/Caller: README flow는 session issue, protected profile, refresh exchange, boundary
  failure를 다룬다.

## 검증

```bash
go test -count=1 ./examples/token-refresh-claims/...
go test -race -count=1 ./examples/token-refresh-claims/...
go test -p 1 ./...
make fmt-check
make tidy-check
make vet
make lint
GOFLAGS=-p=1 make ci
git diff --check
rg -n "context\\.TODO\\(|httptest\\.NewRequest\\(|X-Forwarded-For|RealIP|ListenAndServe\\(|panic\\(|secret|token" examples/token-refresh-claims README.md README.ko.md
```

결과:

- `go test -count=1 ./examples/token-refresh-claims/...` -> PASS.
- `go test -race -count=1 ./examples/token-refresh-claims/...` -> PASS.
- `go test -p 1 ./...` -> PASS.
- `make fmt-check` -> PASS.
- `make tidy-check` -> PASS.
- `make vet` -> PASS.
- `make lint` -> deleted issue #44 worktree의 stale path를 `golangci-lint cache clean`으로
  제거한 뒤 PASS.
- `GOFLAGS=-p=1 make ci` -> PASS.
- `git diff --check` -> PASS.
- `rg` review는 expected token/secret documentation과 test fixture만 찾았다. `panic`,
  forwarded-header trust, unbounded server construction issue는 없었다.
- live smoke with `go run ./examples/token-refresh-claims` -> `/healthz`, `/sessions`,
  access-token `/profile`, refresh exchange, refreshed `/profile`, refresh-as-access
  `403 invalid_claims`에서 PASS.

## 잔여 Risk

refresh-token reuse detection은 의도적으로 scope 밖이다. README는 durable session/revocation
storage와 replay monitoring을 production hardening으로 문서화한다.
