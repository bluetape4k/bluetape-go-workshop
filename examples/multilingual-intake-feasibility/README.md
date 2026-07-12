# multilingual-intake-feasibility

[English](README.md) | [한국어](README.ko.md)

Application-shaped feasibility baseline for `textsearch/language` and
`textsearch/japanese`.

The example evaluates a fixed support-intake feed in English, Korean, Japanese,
mixed English/Japanese, short, and unknown input. It reports heuristic language
evidence explicitly and tokenizes only confident, non-mixed Japanese text.

## Package Lesson

The application owns routing policy; bluetape-go owns detection and tokenization
mechanics:

| Component | Owns |
|---|---|
| `textsearch/language` | Three-language Lingua detector, confidence, ISO codes, mixed-language sections, script hints. |
| `textsearch/japanese` | Kagome IPA tokenization, POS metadata, and byte spans into the original UTF-8 text. |
| Intake evaluator | Minimum confidence/length, manual-review reasons, supported tokenizer path, lifecycle choice. |

This is a feasibility baseline, not the final routing API. Issue #119 owns the
multilingual routing lesson and #67 owns the integrated Gin moderation workflow.

## Behavior

The default policy uses English, Korean, and Japanese models with:

- minimum confidence `0.70`;
- minimum input length `8` runes;
- lazy Lingua model loading;
- one reusable Kagome tokenizer with the IPA dictionary.

Short, unknown, low-confidence, or mixed-script input sets
`manual_review=true` with machine-readable `review_reasons`. English and Korean
can be accepted by the language policy but return `tokenizer="unsupported"`;
the example does not pretend that lexical splitting is morphological analysis.

Confident Japanese input returns nouns and verbs in source order. Every
`start`/`end` pair is a UTF-8 byte span and can slice the original Go string:

```go
original := report.Text[token.Start:token.End]
```

## Run

Print the deterministic JSON report:

```bash
go run ./examples/multilingual-intake-feasibility
```

Representative Japanese output:

```json
{
  "id": "preview-ja",
  "language": "Japanese",
  "confidence": 1,
  "tokenizer": "kagome-ipa",
  "selected_tokens": [
    {"text": "配送", "start": 0, "end": 6, "pos": "名詞/サ変接続/*/*"}
  ],
  "manual_review": false,
  "review_reasons": []
}
```

Mixed English/Japanese input reports both script hints and detected sections,
does not tokenize, and returns `review_reasons=["mixed_language"]`.

## Test

Run deterministic and shared-instance race tests:

```bash
go test -count=1 ./examples/multilingual-intake-feasibility/...
go test -race -count=1 ./examples/multilingual-intake-feasibility/...
```

The bounded stress test shares one detector/tokenizer pair across 18 calls and
asserts exact completion, language decisions, review state, and Japanese byte
spans.

## Lifecycle and Footprint

- Lazy loading reduces startup work but moves model loading to the first calls.
- `PreloadModels` moves that cost to construction when predictable warm-up is
  more important than startup latency.
- The Kagome IPA dictionary has a meaningful binary and memory footprint even
  though only Japanese input uses it.
- Construct detector/tokenizer instances once and reuse them; do not rebuild
  language models for each request.

## Boundaries

- Language and confidence are heuristic evidence, not authentication,
  authorization, policy, or compliance decisions.
- Mixed-language sections are inspectable evidence; this example does not claim
  perfect code-switching segmentation.
- English and Korean tokenization are intentionally unsupported here.
- No HTTP service, model call, storage, or extra tokenizer/detector dependency
  is introduced.
