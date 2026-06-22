# probabilistic-dedupe-admission

[English](README.md) | [한국어](README.ko.md)

Gin webhook admission example for `probabilistic`.

This example uses an in-memory Bloom filter from `bluetape-go/probabilistic` to
prefilter event IDs before an authoritative workflow. It demonstrates
definitely-new events, probably-seen events, false-positive tradeoffs, and why a
production service still needs durable storage.

## Scenario

A checkout webhook receiver gets event IDs from an upstream payment or order
system. The receiver wants a cheap front-door signal before doing heavier
processing. A Bloom filter can say an event ID is definitely new because Bloom
filters have no false negatives for inserted values. It can also say an ID is
probably seen, but that result may be a real duplicate or a false positive.

The demo rejects `probably_seen` events at the prefilter boundary so the behavior
is visible. Production systems should check an authoritative durable store before
dropping important events.

## What It Demonstrates

- In-memory `probabilistic.NewStringBloomFilter` use inside an HTTP service.
- Deterministic first-event `admit` and repeat-event `probably_seen` paths.
- Approximate Bloom filter stats for expected insertions, target false-positive
  probability, approximate element count, and expected current false-positive
  probability.
- Stable public HTTP errors for malformed requests and missing event IDs.
- Production caveat: probabilistic filters are prefilters, not the durable source
  of truth.

## Run

```bash
go run ./examples/probabilistic-dedupe-admission
```

The service listens on `127.0.0.1:8099` by default.

```bash
curl http://127.0.0.1:8099/healthz
```

Admit a new event:

```bash
curl -s -X POST http://127.0.0.1:8099/events/admit \
  -H 'Content-Type: application/json' \
  -d '{"event_id":"evt-1001","source":"checkout"}' | jq
```

Expected decision: `admit`, reason: `definitely_new`, accepted: `true`.

Submit the same event again:

```bash
curl -s -X POST http://127.0.0.1:8099/events/admit \
  -H 'Content-Type: application/json' \
  -d '{"event_id":"evt-1001","source":"checkout"}' | jq
```

Expected decision: `probably_seen`, reason:
`might_be_duplicate_or_false_positive`, accepted: `false`.

Inspect the filter:

```bash
curl -s http://127.0.0.1:8099/filters/current | jq
```

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Process liveness only. |
| `POST` | `/events/admit` | Admit one event ID through the Bloom prefilter. |
| `GET` | `/filters/current` | Inspect approximate Bloom filter stats. |

## Boundary Notes

- `probably_seen` is not proof of duplication. It can be a false positive.
- This example has no durable store. Production dedupe should pair the Bloom
  prefilter with an authoritative table, log, or idempotency store.
- Bloom filters do not delete individual entries. Use expiry windows, rotation,
  or a backend-backed design when that matters.
- The demo key is the trimmed `event_id`; `source` is scenario metadata only.

## Test

```bash
go test -count=1 ./examples/probabilistic-dedupe-admission/...
go test -race -count=1 ./examples/probabilistic-dedupe-admission/...
```
