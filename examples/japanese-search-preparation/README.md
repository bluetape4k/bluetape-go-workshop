# Japanese Search Preparation

[English](README.md) | [한국어](README.ko.md)

Application-shaped Japanese product-catalog preparation for
`textsearch/japanese` and `textsearch`.

A catalog search cannot safely treat Japanese product text as whitespace-separated
words. This example shows how an application turns titles and support text into an
inspectable term index, removes a masked support term from that index, and applies
deterministic all-term matching without hiding token or source-span semantics.

## Package Lesson

The application combines published bluetape-go packages and owns the catalog policy:

| Component | Owns |
|---|---|
| `textsearch/japanese` | Kagome IPA tokenization in Search mode, base forms, POS metadata, and original UTF-8 byte spans. |
| `textsearch` | NFC normalization, the compiled blockword dictionary, masking, and deterministic term matching. |
| `catalogprep.Service` | Noun/verb selection, masked-token exclusion, index projection, all-term hit policy, and lifecycle. |

The lesson has three stages:

1. **Prepare:** tokenize each title and support field, retain nouns and verbs, mask
   configured support substrings, exclude overlapping tokens, and build unique
   NFC-normalized index terms from base forms or surface text.
2. **Search:** prepare the query with the same tokenizer and projection, then require
   every unique query term to match the product's space-delimited index.
3. **Reuse and preview:** construct one service, reuse its tokenizer, dictionary,
   and prepared catalog, and return copied products in a deterministic JSON preview.

## Representative Output

Run the command to print the complete deterministic preview:

```bash
go run ./examples/japanese-search-preparation
```

The actual `JP-200` output includes this title token:

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

`start=9` and `end=15` are start-inclusive/end-exclusive UTF-8 byte offsets in
the original `title` field, not offsets into the support text or prepared index.
For a base-form projection, the support token `温め` has normalized text `温め`,
Kagome POS `動詞/自立/*/*`, base form `温める`, and index term `温める`.

The support policy finds `偽物` at byte span `[48,54)`, produces
`電子レンジで温めて使用できます。**に注意してください。`, retains the token as
`indexable=false`, and omits `偽物` from this index:

```text
ガラス 保存 容器 電子 レンジ 温める 使用 できる 注意 する くださる
```

The preview searches are also fixed:

| Query | Prepared query terms | Result SKUs |
|---|---|---|
| `ランニング シューズ` | `ランニング`, `シューズ` | `JP-100` |
| `保存 容器` | `保存`, `容器` | `JP-200` |
| `宇宙船` | `宇宙`, `船` | none |

## Why Kagome Search Mode

Kagome Normal mode performs regular segmentation. Search mode adds heuristic
segmentation intended for search, which fits an index-preparation lesson where a
compound may need more searchable terms. It does not provide ranking or semantic
search. The example still makes noun/verb selection, normalization, deduplication,
and all-term matching explicit application policy.

## Lifecycle, Normalization, and Boundaries

`NewService` constructs one Search-mode Kagome tokenizer and one blockword
dictionary, prepares the catalog once, and reuses all three across calls. The IPA
dictionary adds startup work plus binary size and memory footprint, so the tokenizer
does not belong in the query path; this example intentionally makes no universal
numeric latency or memory claim. Each `Search` call compiles its matcher locally,
so mutable query state is not shared.

NFC normalization is the comparison and index representation. Token spans keep the
original field's UTF-8 byte positions, including when the original uses a decomposed
representation such as `ガラス`; they are not rune, display-column, normalized-text,
masked-text, or index-text offsets.

Masking uses `BoundaryNone` because Japanese support text does not reliably expose
whitespace word boundaries. Catalog matching instead uses `BoundaryUnicodeWord`
over explicitly space-delimited prepared terms, preventing a query term from
matching inside another prepared term. Neither boundary mode is a semantic
moderation or security boundary.

## Test

Run the focused normal and race tests:

```bash
go test -count=1 ./examples/japanese-search-preparation/...
go test -race -count=1 ./examples/japanese-search-preparation/...
```

The bounded reuse proof shares one service across 6 workers and 6 tasks for 3
rounds per task. It asserts 18 completed calls and actual overlap with
`MaxConcurrent >= 2`; it does not promise a scheduler-specific maximum.

## Boundaries

- Hits are deterministic all-term matches sorted by SKU, not relevance ranking.
- The catalog is in memory; there is no persistence, update pipeline, or external
  search engine.
- There is no HTTP endpoint, Docker dependency, model call, or semantic moderation.
- The example adds no public bluetape-go library API; its service is local under
  `internal/catalogprep`.
- Source positions are UTF-8 byte spans, not rune or display-column offsets.
