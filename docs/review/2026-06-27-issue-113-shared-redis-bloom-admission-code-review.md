# Code review: issue #113 shared Redis Bloom admission

## 범위

- 새 runnable example: `examples/shared-redis-bloom-admission`
- architecture, cross-instance sequence, decision policy를 위한 새 README diagram
- root README navigation과 focused run instruction
- Redis-backed Bloom admission example lesson note

## 발견 사항

local review pass에서 P0/P1 finding은 없다.

## 점검

- `probably_seen`은 authorization이나 exact dedupe로 설명되지 않는다.
- first insert, repeated event, cross-instance shared-state behavior는 Redis-backed test로
  다룬다.
- config fingerprint mismatch는 incompatible Bloom sizing을 조용히 섞지 않고
  `filter_config_mismatch`로 드러낸다.
- Redis outage와 canceled request는 `filter_unavailable`로 mapping된다.
- HTTP server는 기본적으로 loopback에 bind하고 non-loopback `HTTP_ADDR` 값을 거부한다.
- README diagram은 PNG로 render되며 architecture, sequence, policy concern을 분리한다.

## 잔여 Risk

runtime example은 manual `go run`을 위해 externally supplied Redis instance를 기대한다.
test coverage는 Testcontainers Redis를 사용하지만, repository example은 이미 integration
verification에 Testcontainers fixture를 공유하므로 bundled docker-compose file은 포함하지 않는다.
