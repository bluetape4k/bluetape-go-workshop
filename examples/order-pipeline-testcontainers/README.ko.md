# order-pipeline-testcontainers

[English](README.md)

`testcontainers`를 사용한 order pipeline integration 예제입니다.

이 테스트는 `bluetape-go` fixture로 PostgreSQL, Redis, NATS를 시작하고,
hard-coded port 대신 fixture가 반환한 connection string을 사용합니다. 작은 order
projection을 쓰고, idempotency marker를 저장하고, lightweight event를 publish합니다.
이 예제를 실행하려면 Docker가 필요합니다.

## Scenario

![Order pipeline Testcontainers topology](../../docs/images/readme-diagrams/order-pipeline-testcontainers-topology.png)

여러 실제 service를 가로지르는 wiring을 integration test로 증명해야 할 때
사용할 수 있는 흐름입니다. application behavior는 작게 유지하지만, 중요한 운영
계약을 검증합니다: database projection, Redis idempotency, NATS publication이 모두
fixture-provided connection detail을 사용합니다.

## What It Demonstrates

- 하나의 테스트에서 PostgreSQL, Redis, NATS Testcontainers fixture 사용.
- hard-coded port 대신 반환된 connection string 사용.
- `database/sql`을 통한 order projection write.
- Redis idempotency marker.
- NATS event publication과 subscription assertion.

## Run

Docker가 필요합니다.

```bash
go test -count=1 ./examples/order-pipeline-testcontainers/...
```
