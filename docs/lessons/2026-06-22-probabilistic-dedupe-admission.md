# Probabilistic Dedupe Admission Lesson

Issue: #46

`examples/probabilistic-dedupe-admission` demonstrates a Bloom-filter prefilter
with `github.com/bluetape4k/bluetape-go/probabilistic`.

## Decision

Use the in-memory goroutine-safe Bloom filter from `bluetape-go/probabilistic`
for a webhook event admission scenario. The example deliberately names repeat
hits as `probably_seen`, not `duplicate`, because a Bloom filter can produce
false positives.

## Boundaries

- The prefilter key is the trimmed `event_id`.
- First-seen event IDs are admitted as `definitely_new`.
- Repeat or false-positive hits are surfaced as `probably_seen`.
- The example has no durable store and documents that production dedupe needs an
  authoritative table, log, or idempotency store before dropping important data.

## Follow-Up

Issue #78 can compose this example with checkout guards by treating
`probably_seen` as a prefilter signal that must still be reconciled against a
durable workflow boundary.
