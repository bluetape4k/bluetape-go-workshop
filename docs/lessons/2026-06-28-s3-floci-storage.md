# S3 Floci Storage 예제 Lesson

## 맥락

Issue #59는 v0.7.0 workshop track에 첫 S3-shaped storage example을 추가한다. 목표는 전체
AWS SDK를 감싸는 것이 아니다. 유용한 lesson은 receipt object 주변의 작은 application
boundary다. 여기에는 safe key construction, metadata ownership, body close responsibility,
typed missing-object mapping, tenant prefix listing, caller-owned presigned GET URL이
포함된다.

## 결정

예제는 storage API를 좁게 유지하고 각 operation에 caller-owned AWS SDK v2 client를 전달한다.
local smoke coverage는 path-style addressing과 endpoint override를 사용하는
`testcontainers/floci`를 쓴다. README는 real AWS도 같은 request shape를 사용하지만 normal
credential, IAM, networking, encryption, observability control은 예제 밖에서 제공된다고
설명한다.

## 기각한 선택

- 모든 S3 feature를 덮는 broad repository wrapper. reader가 이해해야 하는 AWS SDK contract를
  숨긴다.
- workshop gate의 real AWS integration test. cloud credential이 필요하고 cost/account-state
  concern이 non-deterministic해진다.
- 같은 예제에서 S3 storage를 SQS 또는 DynamoDB와 섞는 방식. S3 object storage는 prerequisite
  lesson이므로 event나 index projection을 도입하기 전에 readable해야 한다.

## 검증 형태

- fake-client test는 Docker 없이 request shape, metadata, safe-key validation,
  `NoSuchKey` mapping, nil body defense, body closing, delete, list, presign TTL
  behavior를 증명한다.
- opt-in Floci smoke test는 Docker가 있을 때 local emulator에 대한 AWS SDK v2 path-style
  S3 call을 증명한다.
- README diagram은 static ownership boundary와 operation sequence를 나눠 reader가 local
  Floci와 real AWS deployment 차이를 모두 이해할 수 있게 한다.
