# Japanese Search Preparation

[English](README.md) | [한국어](README.ko.md)

`textsearch/japanese`와 `textsearch`로 Japanese product catalog를 준비하는
application-shaped 예제입니다.

Japanese product text를 공백으로만 나눠서는 catalog search용 term을 안정적으로
만들 수 없습니다. 이 예제는 title과 support text를 확인 가능한 term index로
투영하고, mask된 support term을 index에서 제외한 뒤 token과 원본 span의 의미를
숨기지 않으면서 deterministic all-term matching을 적용합니다.

## Package Lesson

Application은 released bluetape-go package를 조합하고 catalog policy를 소유합니다.

| Component | Owns |
|---|---|
| `textsearch/japanese` | Search mode의 Kagome IPA tokenization, base form, POS metadata, 원본 UTF-8 byte span. |
| `textsearch` | NFC normalization, compiled blockword dictionary, masking, deterministic term matching. |
| `catalogprep.Service` | Noun/verb 선택, masked token 제외, index projection, all-term hit policy, lifecycle. |

처리는 세 단계로 나뉩니다.

1. **Prepare:** title과 support field를 tokenize하고 noun과 verb만 남깁니다.
   설정된 support substring을 mask하고 겹치는 token을 제외한 뒤, base form 또는
   surface text로 unique NFC-normalized index term을 만듭니다.
2. **Search:** 같은 tokenizer와 projection으로 query를 준비한 다음, unique query
   term이 product의 space-delimited index에 모두 있는지 확인합니다.
3. **Reuse and preview:** service 하나를 생성해 tokenizer, dictionary, prepared
   catalog를 재사용하면서 product copy와 deterministic JSON을 반환합니다.

## Representative Output

전체 deterministic preview를 출력합니다.

```bash
go run ./examples/japanese-search-preparation
```

실제 `JP-200` 출력에는 다음 title token이 들어 있습니다.

```json
{
  "field": "title",
  "text": "保存",
  "normalized": "保存",
  "base_form": "保存",
  "pos": "word",
  "start": 9,
  "end": 15,
  "indexable": true,
  "metadata": {
    "kagome.pos": "名詞/サ変接続/*/*"
  }
}
```

`start=9`, `end=15`는 원본 `title` field에서 시작을 포함하고 끝을 제외하는
UTF-8 byte offset입니다. Support text나 prepared index의 offset이 아닙니다.
Base-form projection의 예로 support token `温め`는 normalized text `温め`,
Kagome POS `動詞/自立/*/*`, base form `温める`, index term `温める`를 가집니다.

Support policy는 `[48,54)` byte span의 `偽物`을 찾고
`電子レンジで温めて使用できます。**に注意してください。`를 만듭니다. 이 token은
`indexable=false`인 evidence로 남지만 다음 index에서는 빠집니다.

```text
ガラス 保存 容器 電子 レンジ 温める 使用 できる 注意 する くださる
```

Preview의 search 결과도 고정되어 있습니다.

| Query | Prepared query terms | Result SKUs |
|---|---|---|
| `ランニング シューズ` | `ランニング`, `シューズ` | `JP-100` |
| `保存 容器` | `保存`, `容器` | `JP-200` |
| `宇宙船` | `宇宙`, `船` | 없음 |

## Why Kagome Search Mode

Kagome Normal mode는 일반 segmentation을 수행합니다. Search mode는 search에
유용하도록 heuristic segmentation을 추가하므로 compound에서 더 많은 검색 term이
필요할 수 있는 index preparation lesson에 맞습니다. Ranking이나 semantic search를
제공하는 것은 아닙니다. Noun/verb 선택, normalization, deduplication, all-term
matching은 여전히 application policy로 명시합니다.

## Lifecycle, Normalization, and Boundaries

`NewService`는 Search-mode Kagome tokenizer 하나와 blockword dictionary 하나를
만들고 catalog를 한 번 준비한 뒤 세 값을 호출 사이에서 재사용합니다. IPA
dictionary는 startup work와 binary/memory footprint를 추가하므로 tokenizer를 query
path에서 만들지 않습니다. 이 예제는 모든 환경에 적용되는 latency나 memory 수치를
제시하지 않습니다. 각 `Search` 호출은 matcher를 호출 안에서 compile하므로 mutable
query state를 공유하지 않습니다.

NFC normalization은 비교와 index에 사용하는 representation입니다. 원본이
`ガラス`처럼 decomposed representation을 사용하더라도 token span은 원본 field의
UTF-8 byte position을 유지합니다. Rune, display column, normalized text, masked text,
index text의 offset이 아닙니다.

Japanese support text는 whitespace word boundary가 항상 드러나지 않으므로 masking은
`BoundaryNone`을 사용합니다. 반면 catalog matching은 명시적으로 공백으로 구분한
prepared term에 `BoundaryUnicodeWord`를 적용해 query term이 다른 prepared term
내부에서 match되지 않게 합니다. 두 boundary mode 모두 semantic moderation이나
security boundary가 아닙니다.

## Test

Focused normal test와 race test를 실행합니다.

```bash
go test -count=1 ./examples/japanese-search-preparation/...
go test -race -count=1 ./examples/japanese-search-preparation/...
```

Bounded reuse proof는 service 하나를 6 workers와 6 tasks가 공유하고 task마다 3
rounds를 실행합니다. 18회 완료와 `MaxConcurrent >= 2`인 실제 overlap을 검증하지만,
scheduler별 특정 maximum을 보장하지는 않습니다.

## Boundaries

- Hit는 SKU로 정렬한 deterministic all-term match이며 relevance ranking이 아닙니다.
- Catalog는 memory에만 있으며 persistence, update pipeline, external search engine은
  없습니다.
- HTTP endpoint, Docker dependency, model call, semantic moderation을 추가하지 않습니다.
- Public bluetape-go library API를 추가하지 않으며 service는
  `internal/catalogprep`에만 있습니다.
- Source position은 UTF-8 byte span이며 rune 또는 display-column offset이 아닙니다.
