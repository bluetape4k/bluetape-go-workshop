# leader-redis-web

[English](README.md) | [한국어](README.ko.md)

Minimal chi-based HTTP service that uses `bluetape-go` Redis leader election.

## Scenario

![Leadership coordination topology](../../docs/images/readme-diagrams/leadership-coordination-topology.png)

Use this example when one HTTP instance should expose and control Redis-backed
leadership. The service keeps the API small so the leader lifecycle is visible:
health check, current leadership state, campaign, and resign.

## Run

```bash
export REDIS_ADDR=localhost:6379
go run ./examples/leader-redis-web
```

## API

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Health check. |
| `GET` | `/leader` | Current leader token and local leadership flag. |
| `POST` | `/campaign` | Try to acquire leadership. |
| `POST` | `/resign` | Release leadership when this instance owns it. |

## What It Demonstrates

- Redis-backed `leader.Elector` wiring in a chi service.
- Request middleware that remains compatible with `net/http`.
- Explicit leadership acquisition and release operations.
- A small API surface for local smoke testing.
