# Issue #44 설계: ID and JWT Boundary 예제

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: issue #44는 첫 번째 구체적인 v0.6.0 예제이며, 이후 issue #76과
  umbrella #78이 연결할 수 있는 ID/JWT trust-boundary pattern을 정립한다.
- 저장소: `bluetape4k/bluetape-go-workshop`.
- 브랜치/워크트리: `feat/issue-44-id-jwt-boundary`, 위치는
  `.worktrees/feat-issue-44-id-jwt-boundary`.

## 목표

Service가 `bluetape-go/id`로 internal order identifier를 생성하고,
`bluetape-go/jwt`로 short-lived request token을 발급 및 검증하며, 이 두 관심사를
HTTP trust boundary에서 분리하는 방식을 보여주는 실행 가능한 Gin 예제를 추가한다.

이 예제는 boundary를 분명히 보여줘야 한다.

- ID는 identifier이며 bearer secret이 아니다.
- JWT payload는 서명되고 검증되지만 암호화되지 않는다.
- Public HTTP error는 안정적이며 token, secret, raw parser diagnostic을 leak하지
  않는다.
- Demo는 local repeatability만을 위해 deterministic fixed HMAC key를 사용한다.

## 현재 근거

- `gh issue view 44`는 ID generation과 JWT trust boundary를 위한 portable utility
  예제, valid/expired/invalid token path 테스트, ID shape, README secret-handling
  guidance, 동기화된 English/Korean 문서, root README navigation을 요구한다.
- `go doc github.com/bluetape4k/bluetape-go/id`는 UUID v4/v7, ULID, KSUID,
  KSUID millis, Snowflake helper를 확인한다. Package 문서는 generated ID가
  authentication token이나 secret이 아니라 identifier라고 명시한다.
- `go doc github.com/bluetape4k/bluetape-go/jwt`는 explicit algorithm 및
  KeyChain helper, fixed HMAC provider, `Compose`, `Parse`, `WithSubject`,
  `WithAudience`, `WithExpiresAfter`, `WithExpectedIssuer`,
  `WithExpectedAudience`, `WithExpirationRequired`, `WithParseClock`을 확인한다.
- `jwt/errors.go`는 `ErrInvalidToken`과 `ErrExpiredToken`을 노출한다.
  `TokenError`는 token string을 노출하지 않고 parser failure를 wrap한다.
- `examples/payment-authorization-state`, `examples/operations-report-policy`
  같은 기존 Gin 예제는 local `internal` package layout, `gin.New`,
  `gin.TestMode` 테스트, 안정적인 error DTO, 작은 `main.go` wiring을 보여준다.
- `examples/invitation-codecs`는 security를 과장하지 않는 encoding/transport
  boundary 문서 톤을 제공한다.

## 비목표

- full auth service, OIDC provider, JWKS endpoint, session manager,
  role/permission framework, production key rotation service를 만들지 않는다.
- database, Redis, Testcontainers, external secret manager, 새 의존성을 추가하지
  않는다.
- generated ID를 추측 불가능한 authorization credential로 취급하지 않는다.
- HTTP response에 token, HMAC secret, raw `Authorization` header, library parser
  error를 echo하지 않는다.
- sibling example `internal` package를 import하지 않는다.

## 검토한 접근

### A. 작은 Gin order intake API

`POST /tokens` demo issuer와 `POST /orders` protected order intake endpoint를
가진 `examples/id-jwt-boundary`를 만든다. Issuer는 반복 가능한 workshop command를
위해 fixed local HMAC provider를 사용한다. Order endpoint는 issuer, audience,
expiration, role, scope를 검증한 뒤 internal UUID v7 order ID를 생성한다.

이 접근을 선택한다. Full auth product를 피하면서 예제를 실행 가능하고 issue에
집중하며 application-shaped로 유지한다.

### B. 순수 library-style CLI 예제

CLI는 HTTP concern을 줄이고 ID/JWT helper를 보여줄 수 있지만, issue #44는 external
request-token flow와 trust boundary를 요구한다. HTTP header와 status code가 없으면
token boundary가 지나치게 추상적이다.

워크숍 저장소는 실행 가능한 application example을 위한 곳이므로 기각한다.

### C. Middleware 중심 auth 예제

Reusable Gin middleware는 익숙하지만 워크숍 저장소가 reusable auth framework를
정의한다고 암시할 위험이 있다. 또한 #44가 가르치려는 boundary decision을 숨긴다.

Lesson은 example-local service layer에서 policy를 명시적으로 유지해야 하므로
기각한다.

## 예제

- 경로: `examples/id-jwt-boundary`
- 패키지: `internal/idjwtboundary`
- 실행 entrypoint: `main.go`
- HTTP framework: Gin
- 기본 주소: `127.0.0.1:8096`
- Package dependency focus: `id`, `jwt`, Gin, 표준 라이브러리 `errors`,
  `net/http`, `strings`, `sync`, `time`.

## 시나리오

Internal order intake service는 trusted upstream gateway의 요청을 받는다. Gateway는
customer subject, role, scope, issuer, audience claim을 가진 short-lived JWT를
발급한다. Order service는 요청을 수락하기 전에 token을 검증하고 order와 request
receipt를 위한 internal UUID v7 identifier를 만든다.

Demo flow:

1. `POST /tokens`는 `customer-1001`용 local demo token을 발급한다.
2. Token 없는 `POST /orders`는 `401 missing_token`을 반환한다.
3. Valid token을 가진 `POST /orders`는 `201`과 UUID v7 `order_id`를 반환한다.
4. Expired 또는 malformed token은 안정적인 `401` error code를 반환한다.
5. Signature는 valid지만 `orders:create` scope가 없는 token은 `403 forbidden`을
   반환한다.

## HTTP 계약

Route:

- `GET /healthz`
- `POST /tokens`
- `POST /orders`

Token 요청:

```json
{
  "subject": "customer-1001",
  "role": "customer",
  "scopes": ["orders:create"],
  "ttl_seconds": 900
}
```

Token 응답:

```json
{
  "token_type": "Bearer",
  "expires_in_seconds": 900,
  "token": "<signed-demo-jwt>"
}
```

Order 요청:

```json
{
  "sku": "sku-blue-tape",
  "quantity": 2
}
```

Order 응답:

```json
{
  "order_id": "018f4c7c-0c00-7c00-8000-000000000000",
  "request_id": "018f4c7c-0c00-7c00-8000-000000000001",
  "subject": "customer-1001",
  "role": "customer",
  "scope": "orders:create",
  "sku": "sku-blue-tape",
  "quantity": 2
}
```

Error 응답:

```json
{
  "error_code": "invalid_token",
  "message": "The bearer token could not be verified."
}
```

Public error code:

- `missing_token` -> 401
- `invalid_token` -> 401
- `expired_token` -> 401
- `forbidden` -> 403
- `invalid_request` -> 400
- `internal_error` -> 500

## Domain 계약

- `Service`는 하나의 `jwt.Provider`, 하나의 `id.StringGenerator`, expected
  issuer, expected audience, required role, required scope, clock을 소유한다.
- `IssueToken`은 `iss`, `sub`, `aud`, `exp`, `role`, `scope`를 compose한다.
- `CreateOrder`는 `Bearer` token을 추출하고 issuer/audience/expiration을
  검증하며, `role=customer`와 `scope=orders:create`를 확인하고 order field를
  검증한 뒤 `order_id`와 `request_id`를 생성한다.
- HTTP handler는 domain sentinel error를 public error contract로 mapping한다.
- Service는 status-free test를 위해 in-memory accepted-order slice를 유지할 수
  있지만, public output은 request/response 기반이다. Persistence lesson은 없다.

Sentinel error:

- `ErrMissingToken`
- `ErrInvalidToken`
- `ErrExpiredToken`
- `ErrForbidden`
- `ErrInvalidRequest`

Domain logic에서 반환하는 error는 테스트와 HTTP mapping이 `errors.Is`를 사용할 수
있도록 유용한 곳에서 sentinel을 `%w`로 wrap한다.

## ID 계약

- Internal order/request identifier에는 `id.NewUUIDV7Generator`를 사용한다.
- 테스트는 반환된 ID를 `id.ParseUUID`로 parse하고 UUID version `7`을 검증한다.
- 테스트는 assertion이 wall clock이나 system entropy에 의존하지 않도록 injected
  deterministic generator 또는 deterministic UUID v7 generator setup을 사용한다.
- README는 UUID v7이 time-sortable identifier를 제공하지만 secrecy를 제공하지
  않는다고 설명한다.

## JWT 계약

- Local demo에는 `jwt.NewFixedHMACProvider(jwt.HS256, secret, jwt.WithClock(...),
  jwt.WithKeyIDGenerator(...))`를 사용한다.
- Token은 `jwt.WithIssuer`, `jwt.WithSubject`, `jwt.WithAudience`,
  `jwt.WithExpiresAfter`, `jwt.WithClaim("role", ...)`,
  `jwt.WithClaim("scope", ...)`로 compose한다.
- Token은 `jwt.WithExpectedIssuer`, `jwt.WithExpectedAudience`,
  `jwt.WithExpirationRequired`, `jwt.WithParseClock`으로 parse한다.
- `errors.Is(err, jwt.ErrExpiredToken)`은 `ErrExpiredToken`으로 mapping한다.
- 그 밖의 parse/signature/key failure는 모두 `ErrInvalidToken`으로 mapping한다.
- README는 fixed secret이 local demo value라고 설명한다. 실제 service는 secret
  manager 또는 environment에서 secret을 load하고 key를 rotate한다.

## 테스트 요구사항

- Valid token은 order를 만들고 201을 반환하며 parse 가능한 UUID v7 ID와 verified
  claim의 subject/role/scope를 포함한다.
- Expired token은 401 `expired_token`을 반환한다.
- Malformed token과 다른 key로 서명된 token은 401 `invalid_token`을 반환한다.
- 누락된 `Authorization` header는 401 `missing_token`을 반환한다.
- Required scope가 없는 valid token은 403 `forbidden`을 반환한다.
- Invalid JSON, blank SKU, 양수가 아닌 quantity는 400 `invalid_request`를
  반환한다.
- Public error response는 raw token, demo secret, raw library error text를 절대
  포함하지 않는다.
- ID generator failure는 internal을 leak하지 않고 500 `internal_error`로
  mapping한다.
- README curl snippet은 local service에서 실행 가능하다.

## 문서 요구사항

- `examples/id-jwt-boundary/README.md`를 추가한다.
- `examples/id-jwt-boundary/README.ko.md`를 추가한다.
- 루트 `README.md`와 `README.ko.md` example table을 갱신한다.
- 두 루트 README에 짧은 run section을 추가한다.
- README pair는 다음을 설명해야 한다.
  - generated ID는 identifier이며 authorization token이 아니다.
  - JWT signing은 integrity와 origin assumption을 검증하지만 claim을 암호화하지
    않는다.
  - local fixed HMAC secret은 deterministic demo material일 뿐이다.
  - 실제 배포에는 secret loading, rotation, TLS, logging hygiene, 이 최소 예제
    밖의 authorization policy가 필요하다.

## 검증

집중 gate:

- `go test -count=1 ./examples/id-jwt-boundary/...`
- `go test -race -count=1 ./examples/id-jwt-boundary/...`
- Live smoke with `go run ./examples/id-jwt-boundary`:
  - `/healthz`
  - issue token
  - create order with valid token
  - missing token
  - expired token
  - malformed token
  - forbidden scope

저장소 gate:

- `go test -p 1 ./...`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `GOFLAGS=-p=1 make ci`
- `git diff --check`

## DoD

- Issue #44 acceptance criteria를 구현한다.
- English/Korean 문서를 동기화한다.
- Root README navigation이 새 예제를 포함한다.
- 기존 `bluetape-go/id`와 `bluetape-go/jwt` package import에 필요한 checksum을
  넘어서는 새 의존성은 없다.
- Step 6-R code review가 `docs/review`에 P0=0 및 P1=0을 기록한다.
- PR metadata는 issue #44를 반영한다. Assignee `debop`, milestone `0.6.0`,
  label `enhancement`, `examples`.
