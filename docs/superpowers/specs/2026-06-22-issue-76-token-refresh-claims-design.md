# Issue #76 설계: Token Refresh and Claims Validation 예제

## 프레임

- Issue: #76 `[v0.6.0] Add token refresh and claims validation example`
- Milestone: `0.6.0`
- 브랜치/워크트리: `feat/issue-76-token-refresh`, 위치는
  `.worktrees/feat-issue-76-token-refresh`.
- Base example: #44 `examples/id-jwt-boundary`는 fixed-HMAC JWT boundary, public
  error allowlist, signed-not-encrypted 문서 표현을 이미 정립했다.

## 목표

Short-lived access-token claim을 검증하고, expired/malformed/wrong-claim token을
거부하며, refresh token을 새 access token으로 교환하되 명확한 public error를
반환하는 실행 가능한 Gin 예제 `examples/token-refresh-claims`를 만든다.

이 예제는 access-token 및 refresh-token 사용 주변의 application boundary를
설명한다. OIDC provider, session store, revocation list, JWKS endpoint, reusable
auth middleware가 아니다.

## 근거

- `gh issue view 76`은 실행 가능한 예제, valid/expired/invalid claim 테스트,
  refresh behavior 테스트, #44를 base ID/JWT boundary 예제로 link하는 README
  navigation을 요구한다.
- `go doc github.com/bluetape4k/bluetape-go/jwt`는 `NewFixedHMACProvider`,
  `Compose`, `Parse`, `WithAudience`, `WithClaim`, `WithExpiresAfter`,
  `WithJWTID`, `WithExpectedIssuer`, `WithExpectedAudience`,
  `WithExpirationRequired`, `WithParseClock`, `Reader.RemainingTTL`을 노출한다.
- `examples/id-jwt-boundary`는 upstream JWT parse failure를 local sentinel로
  mapping하고 public response에 raw token, secret, parser diagnostic이 들어가지
  않게 한다.

## HTTP 계약

| Method | Path | 목적 |
|---|---|---|
| `GET` | `/healthz` | Process liveness 전용. |
| `POST` | `/sessions` | Local demo access-token 및 refresh-token pair를 발급한다. |
| `GET` | `/profile` | Access token을 검증하고 verified claim context를 반환한다. |
| `POST` | `/tokens/refresh` | Refresh token을 검증하고 새 access token을 발급한다. |

## JWT 계약

- Deterministic demo secret과 clock injection을 가진
  `jwt.NewFixedHMACProvider(jwt.HS256, secret, ...)` 하나를 사용한다.
- Access token은 다음을 담는다.
  - `iss = token-refresh-claims`
  - `aud = session-api`
  - `sub = request.subject`
  - `token_use = access`
  - `role = customer`
  - `scope = profile:read`
  - `session_id = deterministic generated ID`
  - `jti = deterministic generated ID`
- Refresh token은 다음을 담는다.
  - `iss = token-refresh-claims`
  - `aud = token-refresh`
  - `sub = request.subject`
  - `token_use = refresh`
  - `session_id = same session id as access token`
  - `jti = deterministic generated ID`
- Access token과 refresh token은 expected issuer, token-specific audience,
  required expiration, parse clock으로 parse한다.
- `jwt.ErrExpiredToken`은 `ErrExpiredToken`으로 취급한다. Malformed signature,
  wrong key, parser failure는 `ErrInvalidToken`으로 취급한다. 검증되었지만 claim
  shape가 잘못된 token은 `ErrInvalidClaims`로 취급한다.

## Error 계약

Public response는 다음 형태를 사용한다.

```json
{"error_code":"invalid_token","message":"The token could not be verified."}
```

허용되는 public code:

- `invalid_request` (`400`)
- `missing_token` (`401`)
- `expired_token` (`401`)
- `invalid_token` (`401`)
- `invalid_claims` (`403`)
- `internal_error` (`500`)

Response는 raw token, demo secret, JWT parser text를 echo하면 안 된다.

## 비목표

- Durable session/revocation store 없음.
- Refresh-token reuse detection 또는 rotation family tracking 없음.
- OIDC discovery, JWKS, browser cookie flow 없음.
- 기존 Gin과 `bluetape-go/jwt`를 넘어서는 새 의존성 없음.

## 문서 요구사항

- `examples/token-refresh-claims` 아래에 English/Korean README pair를 추가한다.
- Root English/Korean README table은 예제를 포함한다.
- Root run section은 #44를 base ID/JWT boundary 예제로 link하고, #76이
  access/refresh token-use claim에 집중한다고 설명한다.
- README boundary note는 production refresh-token flow에 durable session/revocation
  storage, key rotation, TLS, secret management가 필요하다고 설명한다.

## 테스트 요구사항

- Valid session은 두 token type을 모두 발급하고 `/profile`은 access token을
  accept한다.
- Expired access token은 `expired_token`을 반환한다.
- Malformed/wrong-key token은 `invalid_token`을 반환한다.
- Wrong `aud`, `token_use`, role, scope를 가진 verified access token은
  `invalid_claims`를 반환한다.
- Valid refresh token은 `/profile`이 accept하는 새 access token을 반환한다.
- Refresh endpoint에 제출된 access token은 `invalid_claims`로 거부한다.
- Bearer access token으로 제출된 refresh token은 `invalid_claims`로 거부한다.
- Public error body는 raw token, demo secret, parser diagnostic을 leak하지 않는다.
- `main.go`는 loopback binding과 bounded server timeout을 유지한다.

## 완료 기준

- Issue #76 acceptance criteria를 구현한다.
- PR body는 #76을 close하고 `## DoD Status`로 끝난다.
- PR metadata는 issue를 반영한다. Assignee `debop`, milestone `0.6.0`, label
  `enhancement`, `examples`.
