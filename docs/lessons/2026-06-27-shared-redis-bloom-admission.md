# Shared Redis Bloom admission 예제

Issue: #113

## 결정

실행 가능한 Gin webhook admission 예제에서 `probabilistic/redis.NewStringBloomFilter`를
사용하고, repository Testcontainers fixture를 통해 실제 Redis-backed test를 실행한다.

## 이유

in-memory Bloom admission example은 local `definitely_new`와 `probably_seen` boundary를
보여주지만 process-to-process visibility는 보여줄 수 없다. 이 예제는 Bloom bit와 metadata를
Redis로 옮겨 같은 namespace를 사용하는 두 application instance가 같은 probabilistic state를
보게 한다.

app은 HTTP request contract, stable public decision, stable error mapping, reader caveat를
소유한다. `bluetape-go/probabilistic/redis`는 Bloom sizing, Redis storage, metadata
fingerprint check, bit operation, approximate stats를 소유한다.

## 검증 형태

- service test는 first insert, repeated value, cross-instance visibility, config
  mismatch, Redis failure, cancellation, invalid request behavior를 assert한다.
- router test는 stable `admit`, `probably_seen`, stats, health, public invalid
  request mapping을 assert한다.
- README diagram은 shared Redis architecture, cross-instance sequence, decision
  policy를 분리해 Bloom hit가 authorization이나 exact dedupe가 아니라 prefilter signal임을
  보여준다.
- Docker-backed test는 shared container contention을 피하기 위해 `-p 1`로 serial 실행한다.
