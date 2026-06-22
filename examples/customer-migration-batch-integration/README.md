# Customer Migration Batch Integration

[English](README.md) | [한국어](README.ko.md)

This example is the v0.5.0 milestone integration scenario. It exposes a local
Gin operations API for a customer migration batch that combines checkpoint
restart, retry/dead-letter handling, leader-guarded scheduling, status/report
inspection, and active-run cancellation.

## Scenario

The migration imports five deterministic customer records with `ChunkSize: 2`
and checkpoint key `customer-migration`.

| Customer | Behavior |
|---|---|
| `cust-1001` | Migrates successfully in the first chunk. |
| `cust-1002` | Migrates successfully in the first chunk. |
| `cust-1003` | Fails transiently once, then succeeds on retry. |
| `cust-1004` | Fails permanently, records one dead letter, and is skipped. |
| `cust-1005` | Migrates during the restart run. |

The visible walkthrough starts a manual run with
`crash_after_new_writes=3`. The first chunk is checkpointed at `NextIndex=2`,
`cust-1003` is written, and the simulated writer crash rolls the checkpoint
back to the previous successful chunk. A leader-held scheduled tick restarts
from that checkpoint, accepts replayed `cust-1003` as a duplicate no-op,
dead-letters `cust-1004`, writes `cust-1005`, and finishes at `NextIndex=5`.

## Built From the Smaller 0.5.0 Examples

| Source example | Integrated here as |
|---|---|
| [`account-migration-checkpoint-restart`](../account-migration-checkpoint-restart) | Restart from the last successful checkpointed chunk. |
| [`chunked-csv-import-checkpoint`](../chunked-csv-import-checkpoint) | Idempotent replay of a boundary record after a writer crash. |
| [`retry-dead-letter-batch-worker`](../retry-dead-letter-batch-worker) | Retry transient records and dead-letter permanent records. |
| [`operations-report-policy`](../operations-report-policy) | Timestamp-free report projection and stable error codes. |
| [`leader-coordination-jobs`](../leader-coordination-jobs) | Leader-guarded scheduled work without durable queue semantics. |

## Run

```bash
go run ./examples/customer-migration-batch-integration
```

Optional environment variables:

| Variable | Default | Purpose |
|---|---:|---|
| `HTTP_ADDR` | `127.0.0.1:8095` | Loopback listening address. Non-loopback binds are rejected because the API is unauthenticated. |
| `LEADER_MODE` | `held` | `held` allows scheduled ticks; `missing` makes `/batch/schedule/tick` return `not_leader`. |

`/healthz` is process liveness only. Use `/batch/status` for operator
diagnosis: active run state, leader state, latest rejection code, checkpoint,
migrated IDs, and dead letters.

## Endpoints

```bash
curl http://127.0.0.1:8095/healthz

curl -X POST http://127.0.0.1:8095/batch/start \
  -H 'Content-Type: application/json' \
  -d '{"run_id":"manual-001","crash_after_new_writes":3}'

curl http://127.0.0.1:8095/batch/status

curl -X POST http://127.0.0.1:8095/batch/schedule/tick \
  -H 'Content-Type: application/json' \
  -d '{"run_id":"scheduled-001"}'

curl http://127.0.0.1:8095/batch/report
```

Useful failure and operator variants:

```bash
# Cancel an active run. A fast local run may already be complete, which returns no_active_run.
curl -X POST http://127.0.0.1:8095/batch/cancel \
  -H 'Content-Type: application/json' \
  -d '{"reason":"operator requested stop"}'

# Reproduce a missing-leader scheduled tick in another terminal:
LEADER_MODE=missing go run ./examples/customer-migration-batch-integration
curl -X POST http://127.0.0.1:8095/batch/schedule/tick \
  -H 'Content-Type: application/json' \
  -d '{"run_id":"scheduled-missing"}'

# Malformed JSON.
curl -X POST http://127.0.0.1:8095/batch/start \
  -H 'Content-Type: application/json' \
  -d '{'

# Blank run ID.
curl -X POST http://127.0.0.1:8095/batch/start \
  -H 'Content-Type: application/json' \
  -d '{"run_id":"   "}'

# Oversized body.
python3 - <<'PY' | curl -X POST http://127.0.0.1:8095/batch/start \
  -H 'Content-Type: application/json' \
  --data-binary @-
import json
print(json.dumps({"run_id": "manual-big", "padding": "x" * 9000}))
PY
```

Completed runs return `200 OK`. The simulated writer crash returns
`409 Conflict` with `writer_crash`. Missing leadership returns `409 Conflict`
with `not_leader`. Caller cancellation returns `408 Request Timeout` with
`request_cancelled`. Malformed JSON, invalid `run_id`, invalid
`crash_after_new_writes`, oversized bodies, missing reports, and idle cancel
requests return stable error codes.

## Operations Contract

Gin owns routing, bounded JSON body decoding, and HTTP status mapping. The
batch engine owns `batch.Step`, `batch.Job`, checkpoint restore/save, retry
policy, skip policy, and idempotent writes. The service owns active-run
guarding, cancellation, defensive snapshots, leader gating, and latest status
projection.

The tiny scheduler loop is intentionally not a durable queue. It executes a
leader-guarded tick while leadership is held and stops on context cancellation.
Tests drive the loop with manual ticks so the example stays deterministic.

HTTP responses expose customer IDs and counts only. Fixture email values stay
inside the internal sink and never appear in public status, report, or error
responses.

## Runbook Notes

- Stop the server with Ctrl-C; it uses bounded graceful shutdown.
- If port `8095` is busy, use another loopback address such as
  `HTTP_ADDR=127.0.0.1:8096`.
- Restarting the process resets the in-memory checkpoint, migrated sink, dead
  letters, and latest report.
- Widening the bind address requires an explicit trusted-network or
  authentication boundary; this workshop API is unauthenticated.

## Production Hardening

A production migration service would need durable checkpoint storage, database
upserts, idempotency keys, a real scheduler or queue, persistent dead-letter
replay, authentication, metrics, structured run lifecycle logs, and recovery
for work that must survive process restarts.

## Test

```bash
go test -count=1 ./examples/customer-migration-batch-integration/...
go test -race -count=1 ./examples/customer-migration-batch-integration/...
```
