# Issue #118 Japanese Search Preparation Lessons

## Scope and Decision

Issue #118 adds an application-shaped command that prepares a small Japanese
product catalog for deterministic term search and support-text masking. The
example composes the released `bluetape-go` v0.18.0 packages and keeps its
service under `internal/catalogprep`; it does not create workshop infrastructure
or a reusable library API.

The accepted design has three explicit stages: prepare product/query terms with
one reusable Japanese tokenizer, mask support text with one compiled dictionary,
and match query-local terms against an immutable prepared catalog. This separation
keeps token source spans, mask spans, and prepared-index positions from acquiring
an accidental shared coordinate system.

## Search Mode and Lifecycle

Kagome Search mode is the right teaching mode because it adds search-oriented
heuristic segmentation for compound Japanese text. It is still only tokenization:
POS selection, base-form projection, normalization, deduplication, all-term
matching, and SKU ordering remain visible application policy.

`NewService` constructs the Search-mode tokenizer and blockword dictionary once,
prepares the catalog once, and reuses all three. The IPA dictionary carries startup,
binary-size, and memory costs, so construction does not belong in `Search`. Each
search compiles a matcher locally, avoiding mutable matcher state shared between
calls. No context, goroutine, timer, I/O, or shutdown contract is invented for this
bounded CPU-only example.

## Representation and Boundary Contracts

The searchable representation and the source-location representation are
deliberately different:

- comparison and index terms use NFC-normalized Kagome base forms, falling back
  to normalized surface text when no base form exists;
- every token span remains field-local, start-inclusive/end-exclusive UTF-8 byte
  offsets into the original title or support text;
- spans never describe normalized text, masked text, index text, runes, or display
  columns.

Masking uses `BoundaryNone` because Japanese support text does not reliably expose
whitespace word boundaries. A blockword match is therefore explicit substring
policy, not semantic moderation or a security boundary. Selected support tokens
that overlap a match remain inspectable but become non-indexable. Search uses
`BoundaryUnicodeWord` over the explicitly space-delimited prepared index so a
query term cannot match inside another prepared term.

## Ownership and JSON Stability

Owned result slices are created as non-nil empty slices. That distinction matters
for stable JSON: no-match output must be `[]`, not `null`. `Products` recursively
copies product slices and token metadata maps, while every search clones its
matched-term slice. Callers can mutate returned values without changing the
prepared catalog observed by concurrent calls.

The masking exclusion repair replaced repeated token-by-match scanning with a
single ordered sweep. Advancing a monotonic match cursor makes the exclusion pass
`O(tokens + matches)` while preserving half-open byte-span overlap semantics.

## Concurrency and Documentation Repairs

The first contention proof could depend on scheduler timing. The repaired test
uses a deterministic arrival barrier: every task announces once, the final arrival
releases all tasks, and the stress harness then proves exactly 18 completions plus
actual overlap (`MaxConcurrent >= 2`). This removes sleep-based timing assumptions
without claiming a scheduler-specific maximum.

English and Korean README reviews also required natural prose rather than literal
structural translation. Both locales now teach the same lifecycle, normalization,
boundary, output, and non-goal facts while using idiomatic language for their
audiences. Representative output is copied from the runnable JSON rather than
hand-maintained as a separate contract.

## Observed Misses and Review Repairs

- Empty result slices initially risked encoding as `null`; constructors and tests
  now require owned non-nil empties.
- A nested overlap scan obscured the intended linear bound; it was replaced with
  the ordered sweep and boundary-focused tests.
- The reuse stress test needed a deterministic contention barrier to prove overlap
  without flakiness.
- Bilingual prose needed separate naturalness reviews after factual parity review.
- The first final verification exposed 19 `revive` documentation findings and one
  `staticcheck` S1009 finding. The implementation lane added package/export comments
  and simplified the assertion before the entire verification sequence was rerun
  from a new commit.
- The plan incorrectly described installed role dispatch as unavailable. The
  corrected contract injects installed native roles into child prompts when the
  spawn schema has no separate `agent_type` field; executor, verifier,
  code-reviewer, and writer lanes were used.

## Fresh Verification Evidence

The behavioral implementation, public documentation, review repairs, and final
proof were verified through `7f21b9bbbf89948bb6e919b7f93d032527b002ea`
on 2026-07-12. The subsequent ledger-refresh commit changes only this lesson and
the plan checklist; it changes no code, tests, dependencies, or public README.
Because that commit cannot name itself, `git log` is the authority for its exact
SHA.

| Command or check | Result | Evidence |
|---|---|---|
| `make fmt-check` | PASS | exit 0, 0.04s |
| `make tidy-check` | PASS | exit 0, 0.11s |
| `make vet` | PASS | exit 0, 0.57s |
| `make lint` | PASS | exit 0, 1.31s, `0 issues.` |
| `go test -count=1 ./examples/japanese-search-preparation/...` | PASS | exit 0, 0.73s; package 0.553s |
| `go test -race -count=1 ./examples/japanese-search-preparation/...` | PASS | exit 0, 6.69s; package 6.489s |
| `make ci` | PASS | exit 0, 60.60s; repository normal and race suites |
| Two `go run` executions | PASS | 0.89s and 0.61s; both 20,889 bytes and SHA-256 `89ce7bc9731a154910dd776500e61014c6c755481ccb73710793602859df7dd6` |
| JSON parsing and marker audit | PASS | scenario/tokenizer, three products/searches, exact mask and SKUs, no-match `[]`, commands, notes, and all required slices non-null |
| Locale/link commands from the plan | PASS | both example READMEs, both root links, run/race commands, and `git diff --check` |
| Module and negative-scope audit | PASS | `go.mod`/`go.sum` unchanged from `origin/develop`; production scan found no HTTP, ranking, persistence, new dependency, goroutine/lifecycle, or rune-offset contract |

The line-by-line audit mapped all 7 spec acceptance rows, all 9 planned behavior
categories, all 6 implementation tasks, and all 6 Definition-of-Done rows to
current code, tests, CLI JSON, or paired documentation. The final performance,
stability, security, operator, developer/API, user/caller, and integration reviews
converged at P0=0 and P1=0.

## Future Guard and Delivery Boundary

Keep the current spec closed while changes remain a small immutable in-memory
catalog with deterministic all-term matching. Reopen the spec before adding HTTP,
ranking, persistence, catalog updates, a new dependency, non-Japanese routing,
rune/display offsets, a shared mutable matcher, or a public compatibility surface.
Any change to tokenization mode, normalization, mask boundary, index encoding, or
span ownership also requires new fixtures and focused normal/race proof.

Delivery stops at the local ledger-refresh commit on top of `7f21b9b`. PR creation,
issue edits, merge, push, branch deletion, and local synchronization are outside
this task. `git log` is the authority for the exact final local commit. Final
severity count: P0=0, P1=0; no unresolved delivery blocker is known.
