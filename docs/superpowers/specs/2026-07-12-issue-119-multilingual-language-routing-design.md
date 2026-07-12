# Issue #119 Multilingual Language Routing Design

## Status

Approved design for Issue #119, based on bluetape-go v0.18.0. Implementation
planning remains blocked until the Type A spec review converges at P0=0/P1=0
and the user reviews this committed artifact.

## Goal

Add an application-shaped command that turns language detector evidence into a
small, deterministic content-routing decision. The example must make English
and Korean share a moderation route, route confident Japanese text with Kana to
a Japanese-tokenization route, and expose unsupported or uncertain input as a
manual-review decision rather than an internal error.

The lesson is routing policy and detector lifecycle. It does not execute a
moderator or tokenizer and does not make security, compliance, or certainty
claims from language detection.

## Context and Current Evidence

Issue #55 established feasibility for English, Korean, and Japanese detection
and Japanese tokenization. Issue #118 then isolated Japanese search preparation.
Issue #119 must not repeat either token projection or search behavior; it owns
the decision layer that a later Gin integration in Issue #67 can borrow.

The relevant bluetape-go v0.18.0 surfaces are:

- `language.NewDetector` with an explicit language subset;
- `language.WithPreloadedLanguageModels` for an opt-in startup lifecycle;
- `Detector.Detect`, `Detector.Confidences`, and `Detector.DetectMultiple`;
- `Detector.Languages` for inspecting the configured subset;
- `language.ContainsLatin`, `ContainsKorean`, `ContainsJapanese`, and
  `ContainsChinese` for Latin, Hangul, Kana, and Han script hints;
- `language.MaxTextLength`, `ErrBlankText`, and `ErrTextTooLong` for the
  upstream validation boundary.

`Confidences` already returns descending values and ISO codes. Mixed-language
sections already preserve start-inclusive/end-exclusive UTF-8 byte offsets and
text slices. The workshop will expose those contracts rather than reproduce
detector logic.

## Chosen Approach

Use one framework-independent `Router` that owns one reusable detector. The
constructor selects English, Korean, Japanese, and Chinese and applies either
lazy or preloaded model initialization. Each request gathers detection,
confidence, section, and script evidence into call-local values before applying
one explicit routing matrix.

This keeps lifecycle and policy together without creating a reusable workshop
framework. The CLI offers `--preload` so the same fixtures can demonstrate both
construction modes while producing identical routing decisions.

## Rejected Alternatives

### Route directly from the top detected language

Rejected because Han-only text can be classified as Japanese or Chinese
without enough script evidence to select a Japanese tokenizer safely. It also
hides low confidence and mixed sections from the caller.

### Execute moderation and Japanese tokenization inside the router

Rejected because Issue #119 teaches selection, not processor behavior. It
would duplicate Issue #55/#118, couple unrelated lifecycle costs, and make the
future Gin example depend on a workshop-internal pipeline.

### Build separate routers for lazy and preloaded models

Rejected because model loading is a detector construction choice, not a
routing-policy variant. Two router implementations would allow route behavior
to drift and would obscure the lifecycle comparison.

### Add another detector or script library

Rejected because bluetape-go v0.18.0 already exposes the required confidence,
section, ISO-code, and script-hint APIs. Another dependency would not improve
the lesson and would add competing semantics.

## Package and Files

The implementation will live under:

```text
examples/multilingual-language-routing/
  main.go
  main_test.go
  README.md
  README.ko.md
  internal/routing/
    router.go
    router_test.go
```

The root `README.md` and `README.ko.md` will link the new example. No dependency,
workflow, module, container, database, or HTTP registration change is required.

## Configuration and Lifecycle

`Config` owns application policy:

```go
type Config struct {
    MinimumConfidence float64 `json:"minimum_confidence"`
    MinimumRunes      int     `json:"minimum_runes"`
    PreloadModels     bool    `json:"preload_models"`
}
```

The default is minimum confidence `0.70`, minimum length `8` runes, and lazy
model loading. `PreloadModels=true` adds
`language.WithPreloadedLanguageModels()` but does not change the selected
language subset or routing rules.

`NewRouter` constructs exactly one detector for English, Korean, Japanese, and
Chinese. The router has no per-call mutable state, goroutines, channels, I/O,
timers, shutdown method, or context contract. The command constructs one router
and reuses it. Lazy loading is the default; `--preload` pays construction cost
up front. Documentation describes this qualitative tradeoff without universal
startup-time or memory numbers.

## Domain Contract

### Inputs

`Request` contains a required caller-owned ID and text. Blank IDs return a local
invalid-request error. The router trims surrounding ID whitespace and returns
that canonical ID; it never trims or rewrites text because section offsets must
slice the original bytes. Blank text and text over the upstream maximum
preserve the bluetape-go sentinel through `%w`. A nonblank input shorter than
`MinimumRunes` is a valid manual-review decision, not an error.

### Output

`Decision` contains:

- request ID and original text;
- detected language name, ISO 639-1/639-3 codes, and top confidence;
- all four configured language confidences in descending order;
- detector-reported sections with language, ISO codes, original byte spans,
  and exact text slices;
- ordered script hints from `latin`, `hangul`, `kana`, and `han`;
- selected route;
- `manual_review` and an ordered, non-nil `review_reasons` slice.

Unknown detection uses an explicit `unknown` language value and empty ISO
codes. Successful slices are non-nil even when empty so the JSON shape remains
stable. The detector owns confidence ordering; the example verifies but does
not re-rank or synthesize probabilities.

### Routes

The route values are:

- `moderation` for confident, non-mixed English or Korean;
- `japanese-tokenization` for confident, non-mixed Japanese with a Kana hint;
- `manual-review` whenever any review reason exists.

Chinese is deliberately present in the detector subset so Han-only input is
visible, but no Chinese processor is supported by this example.

| Evidence | Route | Review reason |
|---|---|---|
| Confident English, non-mixed | `moderation` | none |
| Confident Korean, non-mixed | `moderation` | none |
| Confident Japanese with Kana, non-mixed | `japanese-tokenization` | none |
| Confident Chinese/Han-only | `manual-review` | `unsupported-language` |
| Japanese detection without Kana and with Han | `manual-review` | `ambiguous-cjk-script` |
| Short, unknown, low-confidence, or mixed input | `manual-review` | corresponding ordered reasons |

Route names describe downstream destinations only. The command does not call a
moderator, tokenizer, queue, or HTTP service.

## Decision Flow

For every request, `Route` performs these steps in order:

1. Validate router construction, ID, blank text, and upstream length.
2. Collect script hints in fixed order: Latin, Hangul, Kana, Han.
3. Call `Detect`, `Confidences`, and `DetectMultiple` on the shared detector.
4. Mark input mixed when detector sections contain at least two distinct
   non-`Unknown` languages. An `Unknown` section remains evidence but does not
   create a second language decision. Script hints remain evidence and do not
   independently redefine detector sections.
5. Append review reasons in this exact order:
   `text-too-short`, `language-unknown`, `low-confidence`, `mixed-language`,
   `ambiguous-cjk-script`, `unsupported-language`.
6. If any reason exists, choose `manual-review`; otherwise apply the confident
   English/Korean/Japanese route matrix.

`language-unknown` applies when no language was detected; `low-confidence`
applies only to a detected language below the threshold, so unknown input does
not receive two names for the same uncertainty. The Japanese guard requires
Kana. Any Japanese result without Kana is treated as ambiguous CJK evidence
rather than permission to invoke a Japanese processor; Han is retained as a
separate observable script hint. A Chinese result produces
`unsupported-language`. Reasons are deduplicated and stable even when one
fixture triggers multiple independent guards.

## Go API Shape

The internal package exposes constructor-only use:

```go
func DefaultConfig() Config
func NewRouter(config Config) (*Router, error)
func (r *Router) Route(request Request) (Decision, error)
func NewPreview(preload bool) (Preview, error)
```

The zero value is intentionally unusable. Calling `Route` on a nil or
uninitialized router fails closed with `ErrInvalidConfig` instead of panicking.
All returned slices are caller-owned copies.

`Preview` includes a scenario name, default configuration, a `model_loading`
value of `lazy` or `preloaded`, qualitative lifecycle notes, heuristic
boundaries, fixed default-policy decisions, one explicit low-confidence policy
check, and validation commands. The lifecycle field is the only intentional
difference in lesson behavior between equivalent lazy and preloaded previews;
the serialized `preload_models` configuration field also reflects the selected
mode. Both the default request decisions and low-confidence policy check must
compare equal across model-loading modes.

The dedicated low-confidence check uses the same detector subset and lifecycle
but raises `MinimumConfidence` to `1.0` for the fixed multilingual-script fixture
`support 문의 订单 delivery`. Under pinned v0.18.0 its detected confidence is
below `1.0`, so the check produces `low-confidence` without changing the
default `0.70` policy. The exact confidence is test evidence, not a documented
accuracy guarantee.

## Errors

The package defines:

- `ErrInvalidConfig` for confidence outside `[0,1]`, non-positive minimum
  runes, or nil/uninitialized router use;
- `ErrInvalidRequest` for a blank request ID.

Detector construction and request operations wrap their causes with `%w`.
`language.ErrBlankText` and `language.ErrTextTooLong` remain discoverable with
`errors.Is`. Unknown, short, low-confidence, mixed, ambiguous-CJK, and
unsupported-language outcomes are valid `Decision` values and never errors.

No partial decision is returned for invalid input or detector failure.

## Concurrency Contract

One `Router` and its detector are safe to reuse concurrently. All request
results, reason slices, confidence projections, and section projections are
call-local. Tests will use a worker pool with capacity six, dispatch six requests
per round, and run three rounds for exactly 18 route calls. A bounded entry gate
proves all six goroutines are ready before releasing each round. Exact outcomes,
cross-round determinism, and the race detector prove safe reuse; the test does
not claim to instrument overlap inside the upstream detector. A focused test
process exercises a fresh lazy router before any route call. The same gate also
completes under `GOMAXPROCS=1` without relying on scheduler parallelism.

The same test runs under focused normal and race commands. It does not create
unbounded goroutines, sleep for correctness, or publish latency claims.

## Command Output

Both commands print deterministic indented JSON:

```bash
go run ./examples/multilingual-language-routing
go run ./examples/multilingual-language-routing --preload
```

Default-policy fixtures cover confident English, Korean, Japanese with Kana,
Chinese/Han-only, mixed English/Japanese, short text, and numeric unknown text.
A separately labeled low-confidence policy check uses threshold `1.0` and the
fixed multilingual-script fixture `support 문의 订单 delivery`. Tests lock only
that the pinned v0.18.0 result is detected below the configured threshold and
produces `low-confidence`; its `Unknown` section does not independently create
`mixed-language`. Documentation does not publish the numeric confidence as a
universal detector guarantee.

Within one mode, repeated output is byte-identical. Lazy and preloaded outputs
have identical request decisions; only explicit lifecycle/configuration
metadata differs.

The three public evidence views per request are intentional because this lesson
exposes single-language, confidence-list, and mixed-section evidence together.
Those APIs can repeat detector computation and allocate result projections. The
README must state that production callers should request only the evidence their
policy needs; this example is not a hot-path throughput template.

The command accepts only fixed, non-sensitive fixtures. `Decision` retains the
original request text to prove section spans, so callers embedding the package
shape remain responsible for redaction, access control, and avoiding sensitive
text in logs or telemetry.

## Failure Modes and Guards

1. **Han-only text is sent to a Japanese tokenizer.** Require a Kana hint for
   the Japanese route and send Han-only Japanese/Chinese evidence to manual
   review with an explicit reason.
2. **Uncertainty is hidden as an internal error.** Model short, unknown,
   low-confidence, mixed, ambiguous, and unsupported input as valid decisions;
   reserve errors for invalid requests/configuration or detector failures.
3. **Lazy and preloaded routers drift in behavior.** Construct both from one
   policy path and compare every decision while allowing only lifecycle
   metadata to differ.
4. **Mixed sections expose invalid offsets.** Preserve upstream byte offsets
   and test that every `text[start:end]` equals the returned section text,
   including multibyte Japanese input.
5. **Shared detector reuse mutates caller-visible slices.** Allocate all result
   projections per call and prove exact results under bounded normal/race tests.
6. **Routing is mistaken for authorization or compliance.** State in both
   README locales and preview boundaries that language detection is heuristic
   evidence and cannot gate authentication, authorization, sanctions, or
   compliance decisions.
7. **A configured low-confidence case changes silently after dependency
   upgrades.** Keep the default threshold at `0.70`, isolate a threshold `1.0`
   policy check, and lock only `detected && confidence < threshold` plus ordered
   reasons. A future detector change must force an intentional fixture/policy
   review rather than weakening the assertion.

## Testing

Tests are written before implementation and cover:

1. default and invalid configuration, selected four-language subset, and nil
   router behavior;
2. confident English and Korean sharing the `moderation` route while retaining
   distinct language/ISO metadata;
3. confident Japanese with Kana selecting `japanese-tokenization`;
4. Chinese and Han-only ambiguity selecting manual review with the correct
   reason;
5. blank ID, blank text, and oversized text with `errors.Is` preservation;
6. short, numeric unknown, and mixed default fixtures plus the separate
   threshold `1.0` low-confidence check with exact ordered reasons;
7. four descending confidence entries and stable ISO codes;
8. distinct-language section mixing and exact UTF-8 byte-span slicing;
9. deterministic, non-nil caller-owned slices;
10. lazy/preloaded decision equality and intentional lifecycle metadata
    difference, scoped to option wiring and behavioral equivalence rather than
    in-process startup or memory measurement;
11. bounded shared-router reuse with six-participant release gates, exact
    completion/results, cross-round determinism, and race proof;
12. CLI flag parsing, repeated deterministic JSON, and route equality across
    both model-loading modes.

CLI tests use an injected argument slice and output/error writers. Unknown
flags return a non-zero result with a deterministic error, while `--help`
returns the standard successful usage path. Preview construction failures are
written to stderr and return non-zero without partial JSON on stdout.

Focused validation is:

```bash
go test -count=1 ./examples/multilingual-language-routing/...
go test -race -count=1 ./examples/multilingual-language-routing/...
go run ./examples/multilingual-language-routing
go run ./examples/multilingual-language-routing --preload
```

Repository validation remains:

```bash
make fmt-check
make tidy-check
make vet
make lint
make test
make race
make ci
```

No container-backed test is added. Existing repository-wide heavyweight tests
remain serialized through the authoritative `make ci` gate.

## Compatibility and Migration

This is a new standalone example and changes no bluetape-go API, existing
workshop API, persisted data, wire contract, dependency, or caller behavior.
The root README pair gains navigation only. No migration or rollback procedure
is needed beyond removing the example and navigation entries before merge.

Future examples may copy the policy shape, but they must import bluetape-go
directly rather than treating this `internal` package or preview JSON as a
stable public API.

## Documentation

The example README pair will include:

- package lesson, routing matrix, and data flow;
- run and focused test commands;
- representative route and manual-review output;
- confidence, mixed-section, script-hint, and UTF-8 byte-span semantics;
- lazy versus preloaded lifecycle tradeoff without benchmark claims;
- the deliberate cost of collecting all three public evidence views and the
  production guidance to request only what a policy uses;
- supported and unsupported routes;
- the heuristic, security, and compliance boundary.

The README pair also warns that decisions retain original text for span
inspection and therefore must not be copied into logs or telemetry without the
caller's redaction and access-control policy.

`README.md` remains English and `README.ko.md` remains source-equivalent natural
Korean. The root README pair adds one navigation row and compact run command.
A diagram is N/A: the three-stage linear flow and six-row routing table are
clearer and more inspectable as text for this small example.

## Non-goals

- No HTTP/Gin endpoint; Issue #67 owns integrated web routing.
- No moderator, tokenizer, queue, persistence, database, cache, or container.
- No new detector dependency or copied detector/script algorithm.
- No automatic fallback from unsupported Chinese to Japanese processing.
- No ranking, translation, locale negotiation, or multilingual content merge.
- No detector accuracy, startup latency, or memory benchmark claim.
- No use of detected language for authentication, authorization, sanctions,
  compliance, or other security decisions.

## Acceptance Mapping

| Issue requirement | Design evidence |
|---|---|
| Small language subset | One reusable English/Korean/Japanese/Chinese detector. |
| Confidence lists and explicit low-confidence fallback | Four descending confidence entries plus a separate threshold `1.0` check that preserves the default `0.70` policy. |
| Mixed sections and script hints | Detector sections with exact byte slices; Latin/Hangul/Kana/Han hints. |
| Deterministic routing policy | Ordered reasons and one explicit route matrix. |
| Lazy/preloaded lifecycle comparison | One constructor path, `--preload`, equal decisions, qualitative metadata only. |
| Shared detector race proof | Bounded 6-worker/6-task/3-round test under normal and race commands. |
| English, Korean, Japanese/CJK, short/unknown/mixed coverage | Fixed fixture set and exact route/reason assertions. |
| Bilingual example docs and root navigation | README locale pair plus root navigation pair. |
| Heuristic/security boundary | Preview notes and both README locales explicitly reject security/compliance use. |
| Repository quality | Focused commands followed by the authoritative `make ci` gate. |

## Spec Review

The required native review lanes were dispatched with read-only scopes for
performance, stability, and user/caller perspectives. They did not respond
after repeated bounded waits and an immediate-return request, so the main
session applied the workflow's timeout fallback and completed all six lenses
against this exact artifact.

| Priority | Lens | Evidence | Resolution |
|---|---|---|---|
| P1 | Stability | The original concurrency wording could mean 108 calls while requiring 18. | Defined six requests per round across a pool of up to six workers, for exactly 18 calls over three rounds. |
| P1 | Developer/API | Japanese without Kana and without an explicit fallback could leave the route matrix incomplete. | Any detected Japanese result without Kana now receives `ambiguous-cjk-script`. |
| P1 | Security | `Decision` retains original text but the caller's logging/privacy duty was unstated. | Limited the CLI to fixed fixtures and added explicit redaction, access-control, and telemetry guidance. |
| P2 | Performance | Three detector queries per request could be mistaken for a production hot-path template. | Documented the teaching cost and production guidance to gather only required evidence. |
| P2 | Operator/Ops | CLI error/help behavior and partial-output boundary were not explicit. | Added injected CLI seams, non-zero error behavior, standard help, stderr, and no partial JSON. |
| P2 | User/caller | Canonical ID behavior and lazy/preloaded comparison fields were ambiguous. | Specified trimmed IDs, byte-exact text, equal decisions, and the two explicit lifecycle/config fields. |
| P1 | Evidence integrity | Planning-time v0.18.0 execution disproved the original claim that `문의 注文 support` was below the default `0.70` threshold. | With user approval, preserved default `0.70` and isolated a threshold `1.0` check using stable fixture `support 문의 订单 delivery`; tests assert the relation, not a universal numeric claim. |
| P1 | Performance/stability | The original `maxActive` gate measured waiting goroutines and could not prove overlap inside the detector. | Removed the internal-overlap claim; require a fresh lazy target, six ready participants, exact outcomes, cross-round determinism, and race proof. |

Integration review found no remaining contradiction, unsupported assumption,
or open material user decision. Latest convergence: P0=0, P1=0. The listed P2
items are resolved in this artifact and require no follow-up issue.

## Definition of Done

- The command prints deterministic evidence for every route and review state.
- English and Korean share `moderation` while preserving language metadata.
- Japanese processing is selected only with confident, non-mixed Kana evidence.
- Chinese/Han-only and all uncertain inputs fail closed to manual review.
- Lazy and preloaded routers produce identical decisions.
- Focused normal/race tests and `make ci` pass from fresh executions.
- Both example and root README locale pairs match actual command behavior.
- Type A spec, plan, verifier, six-lane reviews, lesson, PR metadata, CI, and
  final review gates are complete before the explicit merge boundary.
