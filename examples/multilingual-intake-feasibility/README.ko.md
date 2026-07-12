# multilingual-intake-feasibility

[English](README.md) | [한국어](README.ko.md)

`textsearch/language`와 `textsearch/japanese`를 사용하는 application-shaped
feasibility baseline입니다.

고정된 support intake fixture를 English, Korean, Japanese, English/Japanese
혼합, 짧은 입력, unknown 입력으로 평가합니다. Heuristic language evidence를
명시적으로 보여주고 confidence가 충분하며 혼합되지 않은 Japanese text만
tokenize합니다.

## Package Lesson

Application은 routing policy를 소유하고 bluetape-go는 detection/tokenization
mechanics를 소유합니다.

| Component | Owns |
|---|---|
| `textsearch/language` | English/Korean/Japanese Lingua detector, confidence, ISO code, mixed-language section, script hint. |
| `textsearch/japanese` | Kagome IPA tokenization, POS metadata, 원본 UTF-8 text의 byte span. |
| Intake evaluator | Minimum confidence/length, manual-review reason, 지원 tokenizer path, lifecycle 선택. |

이 예제는 feasibility baseline이며 최종 routing API가 아닙니다. #119는
multilingual routing lesson을, #67은 통합 Gin moderation workflow를 소유합니다.

## Behavior

기본 policy는 English, Korean, Japanese model과 다음 설정을 사용합니다.

- minimum confidence `0.70`;
- minimum input length `8` runes;
- lazy Lingua model loading;
- IPA dictionary를 사용하는 재사용 가능한 Kagome tokenizer 하나.

짧거나 unknown, low-confidence, mixed-script인 입력은 `manual_review=true`와
machine-readable `review_reasons`를 반환합니다. English와 Korean은 language
policy에서 accept할 수 있지만 `tokenizer="unsupported"`를 반환합니다. 단순한
lexical splitting을 morphological analysis인 것처럼 숨기지 않습니다.

Confidence가 충분한 Japanese 입력은 noun과 verb를 원본 순서로 반환합니다.
각 `start`/`end`는 UTF-8 byte span이므로 원본 Go string을 직접 slice할 수
있습니다.

```go
original := report.Text[token.Start:token.End]
```

## Run

Deterministic JSON report를 출력합니다.

```bash
go run ./examples/multilingual-intake-feasibility
```

대표 Japanese output은 다음과 같습니다.

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

English/Japanese 혼합 입력은 두 script hint와 detected section을 보여주고,
tokenize하지 않으며 `review_reasons=["mixed_language"]`를 반환합니다.

## Test

Deterministic test와 shared-instance race test를 실행합니다.

```bash
go test -count=1 ./examples/multilingual-intake-feasibility/...
go test -race -count=1 ./examples/multilingual-intake-feasibility/...
```

Bounded stress test는 detector/tokenizer pair 하나를 18회 공유하고 정확한 완료
횟수, language decision, review state, Japanese byte span을 검증합니다.

## Lifecycle and Footprint

- Lazy loading은 startup work를 줄이는 대신 첫 호출에서 model을 load합니다.
- `PreloadModels`는 예측 가능한 warm-up이 startup latency보다 중요할 때 그
  비용을 constructor로 옮깁니다.
- Kagome IPA dictionary는 Japanese 입력에만 사용하더라도 binary와 memory에
  의미 있는 footprint를 추가합니다.
- Detector/tokenizer는 한 번 생성해 재사용하고 요청마다 language model을
  다시 만들지 않습니다.

## Boundaries

- Language와 confidence는 heuristic evidence이며 authentication,
  authorization, policy, compliance decision이 아닙니다.
- Mixed-language section은 inspectable evidence일 뿐 완벽한 code-switching
  segmentation을 보장하지 않습니다.
- English와 Korean tokenization은 이 예제에서 의도적으로 지원하지 않습니다.
- HTTP service, model call, storage, 추가 tokenizer/detector dependency를
  도입하지 않습니다.
