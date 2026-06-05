# leader-coordination-jobs

[한국어](README.ko.md)

Redis leader election examples for a deployment migration gate and a periodic
cache warmer.

Leader election is useful when exactly one healthy instance should run a
coordination task. Prefer queues, schedulers, or workflow engines when every job
must be durably processed or retried independently.

## Scenario

![Leadership coordination topology](../../docs/images/readme-diagrams/leadership-coordination-topology.png)

Use this example when several replicas are running but only one of them should
own a coordination job. The migration gate runs once while leadership is held,
and the cache warmer keeps looping only while the same instance remains leader.

## What It Demonstrates

- Redis-backed `leader.Elector` coordination.
- A one-shot migration gate protected by `Campaign` and `Resign`.
- A periodic cache warmer that stops on cancellation or lost leadership.
- Testcontainers Redis fixtures for realistic coordination behavior.
- `testing/concurrency` helpers for async job validation.

## Run

```bash
go test -count=1 ./examples/leader-coordination-jobs/...
```
