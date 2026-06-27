# distributed-jwt-key-rotation

[English](README.md) | [한국어](README.ko.md)

`bluetape-go/jwt`, `jwt/redis`, `cache`를 조합하는 distributed JWT signing-key
예제입니다.

이 예제는 두 API instance가 Redis를 통해 JWT signing key를 공유하는 상황을
모델링합니다. 한 instance가 `kid`를 강제로 rotation해도 다른 instance는 retention
window 안의 이전 key로 서명된 token을 계속 검증할 수 있습니다. Trusted local reader
cache는 hot parse path를 빠르게 만들지만 distributed key revalidation은 건너뛰지
않습니다.

## Scenario

`Service.IssueToken`은 repository-backed current HMAC key로 access token을
서명합니다. `Service.RotateKey`는 Redis에 새 key를 강제로 저장합니다.
`Service.VerifyToken`은 `NewCachedDistributedProvider`를 사용하므로 반복 검증은
local reader를 재사용할 수 있지만, retained `kid`가 아직 repository에 있는지 먼저
확인합니다.

모든 public service operation은 Redis-backed repository method를 호출하기 전에 짧은
child `context.Context`를 만듭니다. Cancellation과 deadline은 token error로 숨기지
않고 그대로 반환합니다.

## Architecture

- `jwt/redis.New`가 caller-owned Redis key-chain repository를 만듭니다.
- `jwt.NewDistributedHMACProvider`가 shared repository로 JWT를 생성하고 검증합니다.
- `cache.NewMemory[string,*jwt.Reader]`가 `jwt.NewCachedDistributedProvider`의 반복
  parse cache로 동작합니다.
- Gin router는 token 발급, profile 검증, 명시적 key rotation endpoint를 노출합니다.

## Outcomes

| Case | Behavior | Test |
|---|---|---|
| Shared keys | 같은 namespace로 구성한 두 service가 서로의 token을 검증합니다. | `TestServiceSharesRotatedKeysAcrossInstances` |
| Rotation retention | Forced rotation은 current `kid`를 바꾸지만 이전 token은 retained key window 안에서 계속 유효합니다. | `TestServiceSharesRotatedKeysAcrossInstances` |
| Unknown or stale tokens | 다른 namespace에서 서명된 token과 만료 token은 안정적인 public error로 매핑됩니다. | `TestServiceRejectsUnknownKIDAndExpiredToken` |
| Context budget | Cancelled context는 Redis I/O가 caller cancellation을 숨기기 전에 실패합니다. | `TestServiceHonorsCancelledContextBeforeRepositoryIO` |
| Cached verification | Warm verification은 rotation 뒤에도 retained key를 재검증하므로 안정적으로 동작합니다. | `TestRouterUsesCachedProviderWithKeyRevalidation` |

## Run

Redis를 로컬에서 먼저 실행한 뒤 service를 시작합니다.

```bash
export REDIS_ADDR=localhost:6379
go run ./examples/distributed-jwt-key-rotation
```

Token을 발급하고 검증합니다.

```bash
TOKEN=$(curl -s http://127.0.0.1:8099/tokens \
  -H 'Content-Type: application/json' \
  -d '{"subject":"customer-42","scopes":["orders:read"],"ttl_seconds":300}' |
  jq -r .access_token)

curl -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8099/profile
curl -X POST http://127.0.0.1:8099/keys/rotate
curl -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8099/profile
```

테스트는 repository Testcontainers fixture로 Redis를 시작합니다.

```bash
go test -count=1 ./examples/distributed-jwt-key-rotation/...
go test -race -count=1 ./examples/distributed-jwt-key-rotation/...
```
