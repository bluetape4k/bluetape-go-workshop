# leader-group-web

[English](README.md)

Redis `LeaderGroupElector`를 HTTP service로 노출하는 예제입니다.

이 service는 horizontally scaled worker를 위한 bounded leader slot을 보여줍니다.
동시에 최대 `N`개 instance가 coordination job을 실행해야 할 때 이 패턴을 사용할
수 있습니다. 모든 work unit을 durable하게 처리해야 한다면 queue를 사용해야
합니다.

## Scenario

![Leadership coordination topology](../../docs/images/readme-diagrams/leadership-coordination-topology.png)

정확히 하나의 leader가 아니라 작고 제한된 수의 active worker에 job을 분산해야
하는 상황을 보여줍니다. 각 HTTP instance는 group slot을 얻기 위해 campaign하고,
현재 membership을 보고하며, slot을 내려놓아야 할 때 resign합니다.

## Run

```bash
export REDIS_ADDR=localhost:6379
go run ./examples/leader-group-web
```

## API

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Health check. |
| `GET` | `/group` | 현재 group membership과 local ownership flag. |
| `POST` | `/campaign` | bounded group slot 하나를 획득하려고 시도합니다. |
| `POST` | `/resign` | 이 instance가 소유한 local slot을 release합니다. |

## What It Demonstrates

- 최대 `N`개 active leader를 위한 `LeaderGroupElector`.
- `net/http`에 가깝게 유지되는 lightweight chi web service.
- bounded coordination과 durable queueing의 차이.
