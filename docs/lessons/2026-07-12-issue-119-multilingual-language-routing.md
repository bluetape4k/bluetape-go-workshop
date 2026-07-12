# Issue #119: Multilingual Language Routing Lessons

## Context

The workshop needed an application-shaped example that turns
`textsearch/language` evidence into explicit routes without treating language
detection as authorization, compliance, or certainty. It also needed to show
lazy/preloaded lifecycle choices and safe shared detector reuse.

## Decision

Use one English/Korean/Japanese/Chinese detector and keep application policy in
an internal `Router`. English and Korean share `moderation`; Japanese requires
Kana before `japanese-tokenization`; Chinese, mixed, short, unknown, and other
uncertain evidence fail closed to `manual-review`. Review reasons have one
stable order and returned evidence is caller-owned.

Keep the default confidence at `0.70`. Demonstrate fallback separately with a
threshold of `1.0`, so the lesson does not weaken production-like defaults to
manufacture an uncertain result. `--preload` changes only lifecycle/config
metadata, never decisions.

## Surprising Evidence

The original candidate low-confidence fixture was above the default threshold
under `bluetape-go` v0.18.0. The durable test therefore asserts the relation
`detected confidence < configured threshold`, not a universal detector accuracy
claim. The current fixed fixture reports `0.9821181320051728` under the pinned
version, but that number is output evidence rather than an API guarantee.

A ready/release gate can prove bounded first-use pressure, exact completion,
cross-round equality, and race safety. It cannot prove that model evaluation
overlapped internally, so the test deliberately makes no such claim.

## Outcome

The deterministic CLI reports four descending confidences, ISO metadata,
script hints, UTF-8 byte sections, routes, and ordered review reasons for seven
fixed requests plus the separate low-confidence policy check. English and
Korean README pairs explain lifecycle cost, supported routes, and caller-owned
redaction/logging/access-control duties.

## Verification

Fresh successful commands:

```bash
go test -count=1 ./examples/multilingual-language-routing/...
go test -race -count=1 ./examples/multilingual-language-routing/...
go run ./examples/multilingual-language-routing
go run ./examples/multilingual-language-routing --preload
make fmt-check
make tidy-check
make vet
make lint
make ci
git diff --check origin/develop...HEAD
```

## Review Miss and Future Guard

The first implementation passed package tests but failed repository lint
because best-effort stderr writes did not explicitly handle return values.
Future CLI examples should run the repository lint gate immediately after the
CLI seam is green, while still retaining `make ci` as the final proof.

When detector models or fixtures change, preserve the default policy and
review the fixed evidence intentionally. Do not loosen thresholds, remove
fail-closed reasons, or document exact confidence as an accuracy promise merely
to keep historical output unchanged.
