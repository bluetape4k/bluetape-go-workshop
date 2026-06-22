# token-refresh-claims

[English](README.md) | [한국어](README.ko.md)

`jwt`를 사용하는 Gin token boundary 예제입니다.

이 예제는 짧게 살아 있는 access token과 더 오래 살아 있는 refresh token을 발급하고,
보호된 HTTP boundary에서 access-token claim을 검증한 뒤 refresh token으로 새 access
token을 교환합니다. [issue #44](../../docs/lessons/2026-06-22-id-jwt-boundary.md)의
기본 ID/JWT trust-boundary 예제를 기반으로 합니다.

## Scenario

신뢰된 session service가 두 signed demo token을 발급합니다. Caller는 `/profile`에서
access token을 사용합니다. Access token 만료가 가까워지면 caller는 refresh token을
`/tokens/refresh`로 제출해 새 access token을 받습니다.

이 예제는 `aud`와 `token_use` claim으로 access contract와 refresh contract를
분리합니다. Refresh token은 `/profile`을 호출할 수 없고, access token은
`/tokens/refresh`에서 교환될 수 없습니다.

완전한 auth system은 아닙니다. 각 operation에서 application이 어떤 verified token
claim을 받아들일지 결정하는 focused boundary입니다.

## What It Demonstrates

- `bluetape-go/jwt`의 fixed-HMAC local demo access/refresh token.
- Issuer, audience, expiration, `token_use`, role, scope, subject,
  `session_id` claim 검증.
- Missing, invalid, expired, wrong-claim token에 대한 stable public error.
- Raw token, secret, parser diagnostic을 노출하지 않는 error response.
- Stateless demo와 production refresh-token storage/revocation의 경계 문서화.

## Run

```bash
go run ./examples/token-refresh-claims
```

Service는 기본적으로 `127.0.0.1:8097`에서 listen합니다.

```bash
curl http://127.0.0.1:8097/healthz
```

Local demo session을 발급합니다:

```bash
SESSION=$(
  curl -s -X POST http://127.0.0.1:8097/sessions \
    -H 'Content-Type: application/json' \
    -d '{"subject":"customer-1001","role":"customer","scopes":["profile:read"],"ttl_seconds":300}'
)
ACCESS_TOKEN=$(printf '%s' "${SESSION}" | jq -r '.access_token')
REFRESH_TOKEN=$(printf '%s' "${SESSION}" | jq -r '.refresh_token')
```

Access token으로 protected profile을 조회합니다:

```bash
curl http://127.0.0.1:8097/profile \
  -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

Refresh token을 교환합니다:

```bash
NEW_ACCESS_TOKEN=$(
  curl -s -X POST http://127.0.0.1:8097/tokens/refresh \
    -H 'Content-Type: application/json' \
    -d "{\"refresh_token\":\"${REFRESH_TOKEN}\"}" \
  | jq -r '.access_token'
)
curl http://127.0.0.1:8097/profile \
  -H "Authorization: Bearer ${NEW_ACCESS_TOKEN}"
```

Boundary failure를 재현합니다:

```bash
curl http://127.0.0.1:8097/profile

curl http://127.0.0.1:8097/profile \
  -H 'Authorization: Bearer not-a-jwt'

curl http://127.0.0.1:8097/profile \
  -H "Authorization: Bearer ${REFRESH_TOKEN}"

curl -X POST http://127.0.0.1:8097/tokens/refresh \
  -H 'Content-Type: application/json' \
  -d "{\"refresh_token\":\"${ACCESS_TOKEN}\"}"
```

예상 public error code는 `missing_token`, `invalid_token`, `invalid_claims`,
`expired_token`입니다.

## Endpoints

| Method | Path | 목적 |
|---|---|---|
| `GET` | `/healthz` | Process liveness 전용입니다. |
| `POST` | `/sessions` | Local fixed-HMAC access/refresh demo token을 발급합니다. |
| `GET` | `/profile` | Access-token claim을 검증하고 verified context를 반환합니다. |
| `POST` | `/tokens/refresh` | Refresh-token claim을 검증하고 새 access token을 발급합니다. |

## Boundary Notes

- Access token과 refresh token은 이 demo에서 둘 다 signed JWT이지만 audience와
  `token_use` 값이 다릅니다.
- JWT signature는 claim integrity와 signing key 보유를 검증합니다. Token
  소유자에게 claim 값을 숨기지는 않습니다.
- 이 예제는 stateless입니다. Refresh-token reuse detection, token family revocation,
  session persistence를 제공하지 않습니다.
- Production refresh flow에는 durable session/revocation storage, TLS, key rotation,
  secret management, log scrubbing, replay monitoring이 필요합니다.
- Generated ID, bearer token, signed-but-not-encrypted claim에 대한 기본 #44 lesson은
  `examples/id-jwt-boundary`를 참고하세요.

## Test

```bash
go test -count=1 ./examples/token-refresh-claims/...
go test -race -count=1 ./examples/token-refresh-claims/...
```
