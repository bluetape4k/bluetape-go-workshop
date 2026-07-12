# Japanese Search Preparation

[English](README.md) | 한국어

`textsearch/japanese`와 `textsearch`로 일본어 상품 카탈로그를 검색에 맞게
준비하는 애플리케이션 형태의 예제입니다.

일본어 상품명과 안내 문구는 공백만으로 검색어를 나누기 어렵습니다. 이 예제는
title과 support text를 토큰화해 확인 가능한 검색용 index를 만들고, mask된 support
term을 index에서 제외합니다. 각 token이 원문 어디에서 왔는지도 유지한 채 모든
검색어가 일치하는 상품만 일정한 순서로 찾습니다.

## 패키지 구성과 역할

애플리케이션은 공개된 bluetape-go 패키지를 조합하고 카탈로그 정책을 직접
관리합니다.

| 구성 요소 | 담당 역할 |
|---|---|
| `textsearch/japanese` | Kagome IPA Search mode tokenization, base form, POS metadata, 원본 UTF-8 byte span. |
| `textsearch` | NFC 정규화, 컴파일한 blockword dictionary, masking, 항상 같은 결과를 내는 term matching. |
| `catalogprep.Service` | 명사와 동사 선별, masked token 제외, index 구성, all-term hit 정책, 수명 주기. |

처리는 세 단계로 나뉩니다.

1. **준비:** title과 support field를 토큰화하고 명사와 동사만 남깁니다. 설정된
   support substring을 mask하고 겹치는 token을 제외한 뒤, base form이나 surface
   text를 NFC로 정규화해 중복 없는 index term을 만듭니다.
2. **검색:** 같은 tokenizer와 구성 규칙으로 query를 준비합니다. 중복을 제거한
   query term이 공백으로 구분된 상품 index에 모두 있어야 검색 결과에 포함합니다.
3. **재사용과 미리보기:** service 하나를 생성한 뒤 tokenizer, dictionary, 준비된
   catalog를 재사용합니다. 복사한 상품 정보로 실행할 때마다 같은 JSON preview를
   반환합니다.

## 대표 출력

실행할 때마다 같은 전체 preview를 출력합니다.

```bash
go run ./examples/japanese-search-preparation
```

실제 `JP-200` 출력에는 다음 `title` token이 들어 있습니다.

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
UTF-8 byte offset입니다. Support text나 준비된 index의 offset이 아닙니다.
Base form을 적용하는 예로 support token `温め`의 normalized text는 `温め`,
Kagome POS는 `動詞/自立/*/*`, base form과 index term은 `温める`입니다.

Support 정책은 `[48,54)` byte span의 `偽物`을 찾아
`電子レンジで温めて使用できます。**に注意してください。`로 바꿉니다. 이 token은
`indexable=false`인 근거로 남지만 다음 index에서는 빠집니다.

```text
ガラス 保存 容器 電子 レンジ 温める 使用 できる 注意 する くださる
```

Preview의 검색 결과도 항상 같습니다.

| 검색어 | 준비된 검색어 | 결과 SKU |
|---|---|---|
| `ランニング シューズ` | `ランニング`, `シューズ` | `JP-100` |
| `保存 容器` | `保存`, `容器` | `JP-200` |
| `宇宙船` | `宇宙`, `船` | 없음 |

## Kagome Search 모드를 사용하는 이유

Kagome Normal mode는 일반적인 단어 분할을 수행합니다. Search mode는 검색에 유용한
분할을 heuristic으로 추가합니다. 복합어에서 더 많은 검색 term이 필요할 수 있는
검색 index 준비 예제에 이 mode를 선택한 이유입니다. 다만 ranking이나 semantic
search를 제공하지는 않습니다. 명사와 동사 선별, 정규화, 중복 제거, all-term
matching은 애플리케이션 정책으로 명시합니다.

## 수명 주기, 정규화, 경계

`NewService`는 Search mode Kagome tokenizer 하나와 blockword dictionary 하나를
만들고 catalog를 한 번 준비합니다. 이후 호출에서는 세 값을 모두 재사용합니다.
IPA dictionary는 시작 시 해야 할 작업과 바이너리 크기, 메모리 사용량을 늘리므로
tokenizer를 query path에서 만들지 않습니다. 환경마다 달라지는 지연 시간이나
메모리 수치를 이 예제에서 단정하지는 않습니다. 각 `Search` 호출 안에서 matcher를
컴파일하므로 호출마다 달라지는 검색 상태를 공유하지 않습니다.

비교와 index에는 NFC로 정규화한 표현을 사용합니다. 원본이 `ガラス`처럼 분해된
표현을 사용하더라도 token span은 원본 field의 UTF-8 byte position을 유지합니다.
`rune`, display column, normalized text, masked text, index text의 offset이 아닙니다.

일본어 support text는 공백으로 단어 경계가 항상 드러나지 않으므로 masking에는
`BoundaryNone`을 사용합니다. 반면 catalog matching에는 공백으로 구분한 prepared
term과 `BoundaryUnicodeWord`를 사용합니다. 이렇게 하면 query term이 다른 prepared
term 안에서 match되지 않습니다. 두 boundary mode 모두 semantic moderation이나
security boundary가 아닙니다.

## 테스트

이 예제만 대상으로 일반 테스트와 race 테스트를 실행합니다.

```bash
go test -count=1 ./examples/japanese-search-preparation/...
go test -race -count=1 ./examples/japanese-search-preparation/...
```

제한된 재사용 검증에서는 service 하나를 6 workers와 6 tasks가 공유하고 task마다
3 rounds를 실행합니다. 18회 완료와 `MaxConcurrent >= 2`인 실제 동시 실행을
검증하지만 scheduler별 특정 maximum을 보장하지는 않습니다.

## 범위와 제약

- 검색 결과는 SKU로 정렬한 deterministic all-term match이며 relevance ranking이
  아닙니다.
- Catalog는 memory에만 있으며 persistence, update pipeline, external search engine은
  없습니다.
- HTTP endpoint, Docker dependency, model call, semantic moderation은 포함하지 않습니다.
- Public bluetape-go library API를 추가하지 않으며 service는
  `internal/catalogprep`에만 있습니다.
- Source position은 UTF-8 byte span이며 `rune`이나 display-column offset이 아닙니다.
