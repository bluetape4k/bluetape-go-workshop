# Issue #46 Design: Probabilistic Admission and Dedupe Example

## Goal

Add a runnable `examples/probabilistic-dedupe-admission` Gin API that demonstrates event admission and duplicate prefiltering with `github.com/bluetape4k/bluetape-go/probabilistic`.

The example must show the difference between a definitely-new event and a probably-seen event, keep inputs deterministic for tests and docs, and explain that Bloom filters can return false positives and must be paired with durable authoritative storage in production workflows.

## Current Evidence

- Issue #46 asks for a scenario-shaped probabilistic admission or dedupe example with tests for definitely-new and probably-seen paths.
- `bluetape-go` v0.6.2 exposes `github.com/bluetape4k/bluetape-go/probabilistic`.
- `probabilistic.NewStringBloomFilter` provides an in-memory goroutine-safe Bloom filter with `MightContain`, `Put`, `ApproximateElementCount`, and `ExpectedFPP`.
- The current workshop has no probabilistic example yet.
- Recent public HTTP examples use Gin, `/healthz`, loopback-only default binding, and bounded JSON request bodies.

## User Story

As a workshop reader, I can submit webhook event IDs to a small HTTP API and see whether the event is admitted as definitely new or treated as probably seen by the probabilistic prefilter. I can also inspect approximate filter stats and understand when a production service still needs an authoritative store.

## Scope

- Add `examples/probabilistic-dedupe-admission`.
- Add internal package `examples/probabilistic-dedupe-admission/internal/dedupe`.
- Add English and Korean example READMEs.
- Update root English and Korean README navigation.
- Add a short lesson under `docs/lessons`.
- Add full-feature spec, plan, and review artifacts.

## Non-Goals

- Do not add durable storage. The README must explain that production systems still need one.
- Do not implement Redis-backed Bloom filters or distributed dedupe. The current package docs call those later work.
- Do not claim Bloom filters can prove duplicates. They can only say an ID is definitely new or probably seen.
- Do not add a reusable dedupe framework to the workshop.

## API Contract

### Endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Process liveness only. |
| `POST` | `/events/admit` | Admit one event ID through the Bloom prefilter. |
| `GET` | `/filters/current` | Return approximate filter statistics. |

### Request

```json
{
  "event_id": "evt-1001",
  "source": "checkout"
}
```

`source` is accepted for scenario realism and echoed back, but the prefilter key is the trimmed `event_id`.

### Response

```json
{
  "event_id": "evt-1001",
  "source": "checkout",
  "decision": "admit",
  "reason": "definitely_new",
  "accepted": true,
  "stats": {
    "expected_insertions": 1000,
    "target_false_positive_probability": 0.01,
    "approximate_element_count": 1,
    "expected_false_positive_probability": 0.000000000000000006
  }
}
```

Decision values:

- `admit`: the event was definitely new before insertion and can proceed to the authoritative workflow.
- `probably_seen`: the event may have been processed before, or may be a false positive. The demo rejects it at the prefilter boundary but documents that production must check durable storage before dropping important data.

### Public Errors

| Code | HTTP | Meaning |
|---|---:|---|
| `invalid_request` | 400 | Malformed JSON or missing/invalid event fields. |
| `filter_error` | 500 | Unexpected filter setup or runtime error. |

## Acceptance Criteria

- `go test -count=1 ./examples/probabilistic-dedupe-admission/...` passes.
- `go test -race -count=1 ./examples/probabilistic-dedupe-admission/...` passes.
- `go test -p 1 ./...` passes.
- `make ci` passes before merge.
- Tests cover:
  - first event is `admit` / `definitely_new`,
  - repeated event is `probably_seen`,
  - invalid request mapping,
  - stats are deterministic enough for assertions,
  - HTTP success and duplicate paths.
- README.md and README.ko.md explain false positives and production pairing with durable authoritative storage.
- Root English/Korean READMEs link the new example and list `probabilistic`.

## Step 2-R Integrated Review

Native subagent spawning is not available in this Codex surface, so the six review lanes were run as independent main-session checks using the full-feature reference contract.

| Priority | Area | Finding | Required spec edit |
|---|---|---|---|
| P2 | User | A Bloom filter can only report `probably_seen`; wording must not imply duplicate proof. | Add explicit decision vocabulary and non-goal. Done. |
| P2 | Operator | Production pairing with durable storage is a core issue acceptance criterion. | Add README and boundary requirements. Done. |
| P2 | Stability | Shared filter state requires race validation. | Add changed-package race gate. Done. |
| P3 | Developer | `source` should not complicate dedupe key semantics. | State event ID is the prefilter key. Done. |

Final Step 2-R verdict: P0 = 0, P1 = 0.
