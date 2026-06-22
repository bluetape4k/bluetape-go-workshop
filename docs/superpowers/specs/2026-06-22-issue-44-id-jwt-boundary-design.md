# Issue #44 Design: ID and JWT Boundary Example

## Classification

- Work type: Type A - Full Feature.
- Basis: issue #44 is the first concrete v0.6.0 example and establishes the
  ID/JWT trust-boundary pattern that later issue #76 and umbrella #78 can link
  to.
- Repository: `bluetape4k/bluetape-go-workshop`.
- Branch/worktree: `feat/issue-44-id-jwt-boundary` under
  `.worktrees/feat-issue-44-id-jwt-boundary`.

## Goal

Add a runnable Gin example that shows how a service generates internal order
identifiers with `bluetape-go/id`, issues and verifies short-lived request
tokens with `bluetape-go/jwt`, and keeps those two concerns separated at an
HTTP trust boundary.

The example should make the boundary visible:

- IDs are identifiers, not bearer secrets.
- JWT payloads are signed and validated, not encrypted.
- Public HTTP errors are stable and do not leak tokens, secrets, or raw parser
  diagnostics.
- The demo uses a deterministic fixed HMAC key for local repeatability only.

## Current Evidence

- `gh issue view 44` requires a portable utility example for ID generation and
  JWT trust boundaries, tests for valid/expired/invalid token paths, ID shape,
  README secret-handling guidance, synchronized English/Korean docs, and root
  README navigation.
- `go doc github.com/bluetape4k/bluetape-go/id` confirms UUID v4/v7, ULID,
  KSUID, KSUID millis, and Snowflake helpers. Its package docs explicitly state
  generated IDs are identifiers, not authentication tokens or secrets.
- `go doc github.com/bluetape4k/bluetape-go/jwt` confirms explicit algorithm
  and KeyChain helpers plus fixed HMAC providers, `Compose`, `Parse`,
  `WithSubject`, `WithAudience`, `WithExpiresAfter`, `WithExpectedIssuer`,
  `WithExpectedAudience`, `WithExpirationRequired`, and `WithParseClock`.
- `jwt/errors.go` exposes `ErrInvalidToken` and `ErrExpiredToken`; `TokenError`
  wraps parser failures without exposing the token string.
- Existing Gin examples such as `examples/payment-authorization-state` and
  `examples/operations-report-policy` show local `internal` package layout,
  `gin.New`, `gin.TestMode` tests, stable error DTOs, and small `main.go`
  wiring.
- `examples/invitation-codecs` provides the current documentation tone for
  encoding/transport boundaries without overclaiming security.

## Non-Goals

- Do not build a full auth service, OIDC provider, JWKS endpoint, session
  manager, role/permission framework, or production key rotation service.
- Do not add databases, Redis, Testcontainers, external secret managers, or new
  dependencies.
- Do not treat generated IDs as unguessable authorization credentials.
- Do not echo tokens, HMAC secrets, raw `Authorization` headers, or library
  parser errors in HTTP responses.
- Do not import sibling example `internal` packages.

## Approaches Considered

### A. Small Gin order intake API

Create `examples/id-jwt-boundary` with a `POST /tokens` demo issuer and
`POST /orders` protected order intake endpoint. The issuer uses a fixed local
HMAC provider for repeatable workshop commands. The order endpoint validates
issuer, audience, expiration, role, and scope, then generates an internal UUID
v7 order ID.

This is the selected approach. It keeps the example runnable, issue-focused,
and application-shaped while avoiding a full auth product.

### B. Pure library-style CLI example

A CLI could demonstrate ID and JWT helpers with fewer HTTP concerns, but issue
#44 calls for an external request-token flow and trust boundary. Without HTTP
headers and status codes the token boundary would be too abstract.

Rejected because the workshop repository is for runnable application examples.

### C. Middleware-centered auth example

A reusable Gin middleware would be familiar, but it risks implying the workshop
repo is defining a reusable auth framework. It also hides the boundary decisions
that #44 wants to teach.

Rejected because the lesson should keep policy explicit in the example-local
service layer.

## Example

- Path: `examples/id-jwt-boundary`
- Package: `internal/idjwtboundary`
- Runnable entrypoint: `main.go`
- HTTP framework: Gin
- Default address: `127.0.0.1:8096`
- Package dependency focus: `id`, `jwt`, Gin, and standard-library
  `errors`, `net/http`, `strings`, `sync`, and `time`.

## Scenario

An internal order intake service accepts requests from a trusted upstream
gateway. The gateway issues a short-lived JWT with claims for a customer
subject, role, scope, issuer, and audience. The order service validates the
token before accepting the request and creates internal UUID v7 identifiers for
the order and request receipt.

Demo flow:

1. `POST /tokens` issues a local demo token for `customer-1001`.
2. `POST /orders` without a token returns `401 missing_token`.
3. `POST /orders` with a valid token returns `201` and a UUID v7 `order_id`.
4. Expired or malformed tokens return stable `401` error codes.
5. A token with a valid signature but missing `orders:create` scope returns
   `403 forbidden`.

## HTTP Contract

Routes:

- `GET /healthz`
- `POST /tokens`
- `POST /orders`

Token request:

```json
{
  "subject": "customer-1001",
  "role": "customer",
  "scopes": ["orders:create"],
  "ttl_seconds": 900
}
```

Token response:

```json
{
  "token_type": "Bearer",
  "expires_in_seconds": 900,
  "token": "<signed-demo-jwt>"
}
```

Order request:

```json
{
  "sku": "sku-blue-tape",
  "quantity": 2
}
```

Order response:

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

Error response:

```json
{
  "error_code": "invalid_token",
  "message": "The bearer token could not be verified."
}
```

Public error codes:

- `missing_token` -> 401
- `invalid_token` -> 401
- `expired_token` -> 401
- `forbidden` -> 403
- `invalid_request` -> 400
- `internal_error` -> 500

## Domain Contract

- `Service` owns one `jwt.Provider`, one `id.StringGenerator`, the expected
  issuer, expected audience, required role, required scope, and a clock.
- `IssueToken` composes `iss`, `sub`, `aud`, `exp`, `role`, and `scope`.
- `CreateOrder` extracts a `Bearer` token, validates issuer/audience/expiration,
  checks `role=customer` and `scope=orders:create`, validates order fields, then
  generates `order_id` and `request_id`.
- HTTP handlers map domain sentinel errors to the public error contract.
- The service may keep an in-memory accepted-order slice for status-free tests,
  but public output is request/response based; there is no persistence lesson.

Sentinel errors:

- `ErrMissingToken`
- `ErrInvalidToken`
- `ErrExpiredToken`
- `ErrForbidden`
- `ErrInvalidRequest`

Errors returned from domain logic wrap sentinels with `%w` where useful so
tests and HTTP mapping can use `errors.Is`.

## ID Contract

- Use `id.NewUUIDV7Generator` for internal order/request identifiers.
- Tests parse returned IDs with `id.ParseUUID` and verify UUID version `7`.
- Tests use an injected deterministic generator or deterministic UUID v7
  generator setup so assertions do not depend on wall clock or system entropy.
- README states UUID v7 gives time-sortable identifiers but not secrecy.

## JWT Contract

- Use `jwt.NewFixedHMACProvider(jwt.HS256, secret, jwt.WithClock(...),
  jwt.WithKeyIDGenerator(...))` for the local demo.
- Compose tokens with `jwt.WithIssuer`, `jwt.WithSubject`,
  `jwt.WithAudience`, `jwt.WithExpiresAfter`, `jwt.WithClaim("role", ...)`,
  and `jwt.WithClaim("scope", ...)`.
- Parse tokens with `jwt.WithExpectedIssuer`, `jwt.WithExpectedAudience`,
  `jwt.WithExpirationRequired`, and `jwt.WithParseClock`.
- Map `errors.Is(err, jwt.ErrExpiredToken)` to `ErrExpiredToken`.
- Map all other parse/signature/key failures to `ErrInvalidToken`.
- README states the fixed secret is a local demo value; real services load
  secrets from a secret manager or environment and rotate keys.

## Test Requirements

- Valid token creates an order, returns 201, includes parseable UUID v7 IDs, and
  includes the subject/role/scope from verified claims.
- Expired token returns 401 `expired_token`.
- Malformed token and token signed by a different key return 401
  `invalid_token`.
- Missing `Authorization` header returns 401 `missing_token`.
- Valid token without required scope returns 403 `forbidden`.
- Invalid JSON, blank SKU, and non-positive quantity return 400
  `invalid_request`.
- Public error responses never contain the raw token, the demo secret, or raw
  library error text.
- ID generator failure maps to 500 `internal_error` without leaking internals.
- README curl snippets are runnable against the local service.

## Documentation Requirements

- Add `examples/id-jwt-boundary/README.md`.
- Add `examples/id-jwt-boundary/README.ko.md`.
- Update root `README.md` and `README.ko.md` example tables.
- Add a short run section to both root READMEs.
- README pair must explain:
  - generated IDs are identifiers, not authorization tokens;
  - JWT signing validates integrity and origin assumptions but does not encrypt
    claims;
  - local fixed HMAC secret is deterministic demo material only;
  - real deployments require secret loading, rotation, TLS, logging hygiene, and
    authorization policy outside this minimal example.

## Verification

Focused gates:

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

Repository gates:

- `go test -p 1 ./...`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `GOFLAGS=-p=1 make ci`
- `git diff --check`

## DoD

- Issue #44 acceptance criteria are implemented.
- English/Korean docs are synchronized.
- Root README navigation includes the new example.
- No new dependencies beyond checksums required by importing existing
  `bluetape-go/id` and `bluetape-go/jwt` packages.
- Step 6-R code review records P0=0 and P1=0 in `docs/review`.
- PR metadata mirrors issue #44: assignee `debop`, milestone `0.6.0`, labels
  `enhancement` and `examples`.
