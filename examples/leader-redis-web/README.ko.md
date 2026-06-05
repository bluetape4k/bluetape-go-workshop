# leader-redis-web

[English](README.md) | [한국어](README.ko.md)

`bluetape-go` Redis leader election을 사용하는 최소 chi 기반 HTTP service입니다.

## Scenario

![Leadership coordination topology](../../docs/images/readme-diagrams/leadership-coordination-topology.png)

하나의 HTTP instance가 Redis-backed leadership을 노출하고 제어해야 하는 상황을
보여줍니다. 이 service는 leader lifecycle이 잘 보이도록 API를 작게 유지합니다:
health check, 현재 leadership state, campaign, resign.

## Run

```bash
export REDIS_ADDR=localhost:6379
go run ./examples/leader-redis-web
```

## API

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Health check. |
| `GET` | `/leader` | 현재 leader token과 local leadership flag. |
| `POST` | `/campaign` | leadership 획득을 시도합니다. |
| `POST` | `/resign` | 이 instance가 소유한 leadership을 release합니다. |

## What It Demonstrates

- chi service에서 Redis-backed `leader.Elector` wiring.
- `net/http`와 호환되는 request middleware.
- 명시적인 leadership acquisition/release operation.
- local smoke testing에 적합한 작은 API surface.
