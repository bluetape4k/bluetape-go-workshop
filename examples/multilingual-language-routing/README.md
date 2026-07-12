# multilingual-language-routing

English | [한국어](README.ko.md)

## Package Lesson

This application-shaped example separates `textsearch/language` evidence from
application routing policy. One detector is limited to English, Korean,
Japanese, and Chinese; the application decides whether evidence goes to
`moderation`, `japanese-tokenization`, or `manual-review`.

Defaults are `MinimumConfidence=0.70`, `MinimumRunes=8`, and lazy model loading.

## Routing Matrix

| Evidence | Route | Ordered review reasons |
|---|---|---|
| Confident English | `moderation` | none |
| Confident Korean | `moderation` | none |
| Confident Japanese with Kana | `japanese-tokenization` | none |
| Confident Chinese/Han-only | `manual-review` | `unsupported-language` |
| Japanese without Kana | `manual-review` | `ambiguous-cjk-script` |
| Short, unknown, low-confidence, or mixed | `manual-review` | `text-too-short`, `language-unknown`, `low-confidence`, `mixed-language` as applicable |

Reasons always use this order: `text-too-short`, `language-unknown`,
`low-confidence`, `mixed-language`, `ambiguous-cjk-script`,
`unsupported-language`.

## Evidence Contract

Each decision exposes all four detector confidences in descending order with
ISO 639-1/639-3 codes. Script hints are ordered as Latin, Hangul, Kana, and Han.
Mixed-language sections retain the original text and UTF-8 byte spans; for the
fixed mixed fixture the actual spans are English `[0,13)`, Japanese `[13,53)`,
and English `[53,63)`.

## Run

```bash
go run ./examples/multilingual-language-routing
go run ./examples/multilingual-language-routing --preload
```

Representative fields from the deterministic lazy output are:

```json
{
  "config": {"minimum_confidence": 0.7, "minimum_runes": 8, "preload_models": false},
  "model_loading": "lazy",
  "decisions": [
    {"id": "preview-en", "language": "English", "route": "moderation", "review_reasons": []},
    {"id": "preview-ko", "language": "Korean", "route": "moderation", "review_reasons": []},
    {"id": "preview-ja", "language": "Japanese", "route": "japanese-tokenization", "review_reasons": []},
    {"id": "preview-zh", "language": "Chinese", "route": "manual-review", "review_reasons": ["unsupported-language"]},
    {"id": "preview-mixed", "language": "Japanese", "route": "manual-review", "review_reasons": ["mixed-language"]}
  ],
  "low_confidence_fallback": {
    "config": {"minimum_confidence": 1.0, "minimum_runes": 8, "preload_models": false},
    "decision": {
      "id": "preview-low",
      "language": "Korean",
      "confidence": 0.9821181320051728,
      "route": "manual-review",
      "review_reasons": ["low-confidence"]
    }
  }
}
```

The snippet selects fields from actual stdout without changing their values.
The command emits the complete evidence object. `--preload` changes
lifecycle/config metadata only; all decisions remain equal.

## Test

```bash
go test -count=1 ./examples/multilingual-language-routing/...
go test -race -count=1 ./examples/multilingual-language-routing/...
```

The bounded concurrency test releases six ready callers per round for three
rounds: exactly 18 calls, three identical decisions per request, including a
fresh lazy first use and a `GOMAXPROCS=1` completion check.

## Lifecycle and Cost

Construct one detector and reuse it. Lazy loading defers model work; preloading
moves it to construction without changing routes. This example intentionally
collects three public evidence views (`Detect`, `Confidences`, and
`DetectMultiple`), which repeats work and creates projection allocations.
Production code should gather only the evidence it uses. The in-process
comparison proves option wiring and route equality, not startup time, memory
usage, or model-cache performance.

## Boundaries

The command does not execute a moderator, tokenizer, queue, or service.
Language evidence is heuristic, not certainty, and must not control
authentication, authorization, sanctions, or compliance. Decisions retain the
original text for span inspection; callers own redaction, logging, telemetry,
access control, and storage policy.
