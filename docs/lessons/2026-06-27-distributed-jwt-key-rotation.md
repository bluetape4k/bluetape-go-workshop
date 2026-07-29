# Distributed JWT key rotation 예제

Issue: #111

## 결정

distributed repository를 fake store 뒤에 감싸지 않고 application-shaped Gin 예제에서
`jwt/redis.New`, `jwt.NewDistributedHMACProvider`,
`jwt.NewCachedDistributedProvider`를 직접 사용한다.

## 이유

lesson은 JWT composition만이 아니다. 중요한 boundary는 두 API instance가 Redis를 통해
signing authority를 공유하고, 새 `kid`를 강제로 발급하며, local reader cache hit가 있어도
repository key state를 다시 검증하면서 retained key를 계속 verify한다는 점이다.

## 검증 형태

- service test는 repository Testcontainers fixture로 Redis를 시작한다.
- rotation test는 새 `kid` 발급과 old token retention을 모두 assert한다.
- cancellation test는 Redis I/O가 token verification failure로 오해되기 전에 caller
  `context.Context` error가 보이도록 유지한다.
- router test는 cached distributed provider를 통한 warm repeated verification을 실행한다.
