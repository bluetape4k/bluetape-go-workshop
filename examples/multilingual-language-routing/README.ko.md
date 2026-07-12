# multilingual-language-routing

[English](README.md) | 한국어

## Package 학습 목표

이 application-shaped 예제는 `textsearch/language`가 제공하는 근거와 application
routing policy를 분리합니다. Detector는 English, Korean, Japanese, Chinese만
판별하고, application은 그 근거를 `moderation`, `japanese-tokenization`,
`manual-review` 중 어디로 보낼지 결정합니다.

기본값은 `MinimumConfidence=0.70`, `MinimumRunes=8`, lazy model loading입니다.

## Routing Matrix

| 근거 | Route | 순서가 고정된 review reason |
|---|---|---|
| 신뢰도가 충분한 English | `moderation` | 없음 |
| 신뢰도가 충분한 Korean | `moderation` | 없음 |
| Kana가 있는 Japanese | `japanese-tokenization` | 없음 |
| 신뢰도가 충분한 Chinese/Han-only | `manual-review` | `unsupported-language` |
| Kana가 없는 Japanese | `manual-review` | `ambiguous-cjk-script` |
| 짧음, unknown, 낮은 신뢰도, 혼합 언어 | `manual-review` | 상황에 따라 `text-too-short`, `language-unknown`, `low-confidence`, `mixed-language` |

Reason 순서는 항상 `text-too-short`, `language-unknown`, `low-confidence`,
`mixed-language`, `ambiguous-cjk-script`, `unsupported-language`입니다.

## Evidence Contract

각 decision은 내림차순으로 정렬된 네 언어의 confidence와 ISO 639-1/639-3 code를
모두 노출합니다. Script hint 순서는 Latin, Hangul, Kana, Han입니다. 혼합 언어
section은 원문과 UTF-8 byte span을 유지합니다. 고정 mixed fixture의 실제 span은
English `[0,13)`, Japanese `[13,53)`, English `[53,63)`입니다.

## 실행

```bash
go run ./examples/multilingual-language-routing
go run ./examples/multilingual-language-routing --preload
```

Deterministic lazy 출력에서 대표 field만 추리면 다음과 같습니다.

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

이 snippet은 실제 stdout에서 field를 골랐으며 값은 바꾸지 않았습니다. 명령은 전체
근거 object를 출력합니다. `--preload`는 lifecycle/config metadata만 바꾸며 모든
decision은 동일합니다.

## 테스트

```bash
go test -count=1 ./examples/multilingual-language-routing/...
go test -race -count=1 ./examples/multilingual-language-routing/...
```

Bounded concurrency test는 round마다 준비된 caller 6개를 세 번 release합니다.
정확히 18번 호출하고 request마다 동일한 decision을 세 번 확인하며, fresh lazy
first use와 `GOMAXPROCS=1` 완료도 검증합니다.

## Lifecycle과 비용

Detector는 한 번 만들고 재사용합니다. Lazy loading은 model 작업을 미루고,
preloading은 route를 바꾸지 않은 채 그 작업을 construction 시점으로 옮깁니다.
이 예제는 세 public evidence view(`Detect`, `Confidences`, `DetectMultiple`)를
의도적으로 모두 수집하므로 작업이 반복되고 projection allocation이 발생합니다.
Production에서는 실제로 사용하는 근거만 수집해야 합니다. In-process 비교는 option
wiring과 route 동등성만 증명하며 startup time, memory 사용량, model-cache 성능을
측정하지 않습니다.

## 경계

이 명령은 moderator, tokenizer, queue, service를 실제로 실행하지 않습니다.
Language evidence는 heuristic일 뿐 certainty가 아니며 authentication,
authorization, sanctions, compliance를 결정하는 데 사용하면 안 됩니다. Decision은
span 확인을 위해 원문을 유지하므로 caller가 redaction, logging, telemetry, access
control, storage policy를 책임져야 합니다.
