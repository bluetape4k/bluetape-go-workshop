# Issue #44 ID/JWT Boundary Code Review

## 범위

- Branch: `feat/issue-44-id-jwt-boundary`
- Planning commit: `0303d19 Define the ID and JWT boundary before coding`
- 검토 범위:
  - `examples/id-jwt-boundary/**`
  - `README.md`
  - `README.ko.md`
  - `go.mod`
  - `go.sum`
  - `docs/lessons/2026-06-22-id-jwt-boundary.md`

## Step 5 Verifier 결과

PASS.

- Issue #44 acceptance criteria는 구현에 mapping된다.
  - ID generation: `Service.CreateOrder`는 injected `id.NewUUIDV7Generator` default를
    사용하고 test는 `id.ParseUUID`로 ID를 parse한다.
  - JWT issue/verify: `/tokens`는 `jwt.NewFixedHMACProvider`로 sign하고 `/orders`는
    expected issuer, audience, required expiration으로 parse한다.
  - invalid token rejection: test는 missing, malformed, wrong-key, expired,
    forbidden-scope token을 다룬다.
  - trust-boundary docs: example README pair는 ID가 bearer secret이 아니며 signed JWT
    claim이 encrypted data가 아니라고 설명한다.
  - no real secrets: committed HMAC 값은 deterministic demo material 전용으로 문서화되어 있다.
  - root navigation: `README.md`와 `README.ko.md`에는 table/run section entry가 있다.

## Step 4-P Performance/Stability Scan

검토 범위에서 performance 또는 stability issue는 발견되지 않았다.

| Priority | File:Line | Area | Finding | Fix |
|---|---|---|---|---|
| N/A | `examples/id-jwt-boundary/internal/idjwtboundary/service.go:265` | SECURITY/STABILITY | JSON request body는 binding 전에 8 KiB로 제한된다. | N/A |
| N/A | `examples/id-jwt-boundary/internal/idjwtboundary/service.go:262` | SECURITY | Gin trusted proxy는 이 local boundary example에서 비활성화된다. | N/A |
| N/A | `examples/id-jwt-boundary/main.go:58` | OPS/STABILITY | HTTP server는 bounded read-header, read, write, idle timeout을 가진다. | N/A |
| N/A | `examples/id-jwt-boundary/main.go:76` | SECURITY | `HTTP_ADDR`는 기본적으로 non-loopback bind를 거부한다. | N/A |
| N/A | `examples/id-jwt-boundary/internal/idjwtboundary/service.go:369` | SECURITY | public error response는 allowlisted이고 raw token/parser text를 포함하지 않는다. | N/A |

## Step 6-R Six-Lane Review

| Lane | P0 | P1 | P2 | P3 | Result |
|---|---:|---:|---:|---:|---|
| Tier 1 Performance | 0 | 0 | 0 | 0 | PASS |
| Tier 2 Stability | 0 | 0 | 0 | 0 | PASS |
| Tier 3 Security | 0 | 0 | 0 | 0 | PASS |
| Tier 4 Operator/Ops | 0 | 0 | 0 | 0 | PASS |
| Tier 5 Developer/API | 0 | 0 | 0 | 0 | PASS |
| Tier 6 User/Caller | 0 | 0 | 0 | 0 | PASS |

## Tier 메모

- Performance: request handling은 bounded/local이며 background worker, store, network
  client, sleep, retry loop를 추가하지 않는다.
- Stability: test는 clock, ID generator, signing material을 inject한다. entrypoint test는
  bind parsing과 server timeout을 다룬다.
- Security: error mapping은 parser detail을 public code로 축약한다. test는 response body가
  raw token, demo secret, common parser text를 누출하지 않음을 assert한다.
- Operator/Ops: loopback-only default는 unauthenticated demo API를 local로 제한하고 README는
  production hardening boundary를 문서화한다.
- Developer/API: example-local `internal/idjwtboundary`가 reusable auth middleware나 sibling
  `internal` package import 없이 lesson을 소유한다.
- User/Caller: 영어/한국어 README pair는 valid, missing, invalid, forbidden, expired token path에
  대한 runnable curl flow를 제공한다.

## Production Boundary Quick Scan 결과

Command:

```bash
rg -n "context\\.TODO\\(|httptest\\.NewRequest\\(|X-Forwarded-For|RealIP|ListenAndServe\\(|panic\\(|secret|token" examples/id-jwt-boundary README.md README.ko.md
```

결과: intentional hit만 있었다.

- `main.go`: owned `ListenAndServe` lifecycle과 loopback bind validation.
- `service.go`: 고정 HMAC 데모 secret, token 발급/parse 코드, allowlist 처리된 공개 token 오류.
- `service_test.go`: context-aware `httptest.NewRequestWithContext`, test token, leak assertion.
- README files: demo secret은 production material이 아니고 JWT claim은 encrypted data가 아니라는
  explicit warning.

## 검증 Evidence

- `go test -count=1 ./examples/id-jwt-boundary/...` -> PASS.
- `go test -race -count=1 ./examples/id-jwt-boundary/...` -> PASS.
- `go test -p 1 ./...` -> PASS.
- `make fmt-check` -> PASS.
- `make vet` -> PASS.
- `make lint` -> `0 issues.`
- `git diff --check` -> PASS.
- Live smoke:
  - `/healthz` -> 200 `{"status":"ok"}`
  - valid token order -> 201 with UUID v7 `order_id` and `request_id`
  - missing token -> 401 `missing_token`
  - malformed token -> 401 `invalid_token`
  - forbidden scope -> 403 `forbidden`
  - expired token -> 401 `expired_token`

## 수렴 결과

Final gate: P0 = 0, P1 = 0.

final review scope에서 P2/P3 follow-up은 발견되지 않았다.
