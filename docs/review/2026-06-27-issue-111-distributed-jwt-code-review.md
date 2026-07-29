# Code review: issue #111 distributed JWT key rotation

## 범위

- 새 runnable example: `examples/distributed-jwt-key-rotation`
- root README navigation update
- distributed JWT/Redis example lesson note

## 발견 사항

local review pass에서 P0/P1 finding은 없다.

## 점검

- service method는 repository-backed JWT operation 전에 bounded child context를 만든다.
- public error는 token string이나 Redis internal을 누출하지 않는다.
- cache usage는 `jwt.NewCachedDistributedProvider` 뒤에 머물며, warm reader를 반환하기 전에
  retained `kid` state를 다시 검증한다.
- test는 shared-key verification, forced rotation, unknown `kid`, expired token,
  cancellation, rotation 이후 warm cached verification을 다룬다.

## 잔여 Risk

예제는 의도적으로 HMAC key rotation만 보여준다. 0.6.x milestone에서 더 넓은 repository
coverage가 필요하면 RSA와 MongoDB distributed repository는 별도 focused follow-up example로
남긴다.
