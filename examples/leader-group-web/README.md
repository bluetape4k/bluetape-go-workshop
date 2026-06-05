# leader-group-web

[English](README.md) | [한국어](README.ko.md)

Redis `LeaderGroupElector` web example.

The service demonstrates bounded leader slots for horizontally scaled workers.
Use this pattern when up to `N` instances should run a coordination job at the
same time; use a queue when every unit of work needs durable processing.

## Scenario

![Leadership coordination topology](../../docs/images/readme-diagrams/leadership-coordination-topology.png)

Use this example when a job should be distributed across a small, bounded number
of active workers rather than exactly one leader. Each HTTP instance campaigns
for a group slot, reports current membership, and resigns when the slot should
be released.

## Run

```bash
export REDIS_ADDR=localhost:6379
go run ./examples/leader-group-web
```

## API

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Health check. |
| `GET` | `/group` | Current group membership and local ownership flag. |
| `POST` | `/campaign` | Try to acquire one bounded group slot. |
| `POST` | `/resign` | Release the local slot when this instance owns it. |

## What It Demonstrates

- `LeaderGroupElector` for up to `N` active leaders.
- A lightweight chi web service that stays close to `net/http`.
- Clear distinction between bounded coordination and durable queueing.
