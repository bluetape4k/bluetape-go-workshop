# 변경 이력

이 프로젝트의 의미 있는 변경 사항은 이 파일에 기록한다.

형식은 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)를 따르며,
첫 태그가 게시된 뒤에는 semantic versioning을 사용한다.

## [미공개]

### 추가됨

- 실행 가능한 `bluetape-go` 웹 애플리케이션 예제를 담는 초기 workshop 저장소를 추가했다.
- Testcontainers 기반 통합 테스트를 포함한 `chi` 기반 Redis leader election HTTP 예제를 추가했다.
- 캐시되지 않은 container-backed 테스트를 실행하는 CI와 Nightly workflow를 추가했다.
- retry, timeout, circuit breaker, bulkhead, event hook을 보여 주는 resilience HTTP web 예제를 추가했다.
- serialization/compression, core/collections, codec, leader job, concurrency fan-out, Testcontainers, leader group coordination을 다루는 foundation workshop 예제를 추가했다.
- retry, per-attempt timeout, typed error check, event visibility, README diagram을 다루는 catalog refresh resilience 예제를 추가했다.
