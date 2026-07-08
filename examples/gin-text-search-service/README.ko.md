# Gin Text Search Service 예제

[English](README.md) | [한국어](README.ko.md)

이 예제는 deterministic `textsearch` 동작을 작은 Gin API로 노출합니다. Local
moderation masking 예제 다음 단계로, "exact matching은 어떻게 동작하는가?"보다
"HTTP service가 search/masking을 handler에 policy를 넣지 않고 어떻게 노출해야
하는가?"에 초점을 둡니다.

Gin은 JSON binding, status code, public error shape를 소유합니다. 재사용 가능한
domain `Service`는 policy compile, Unicode boundary matching, leftmost-longest
overlap 처리, exact-span masking을 소유합니다.

## Scenario

Content operations tool이 더 큰 moderation workflow 전에 policy phrase가 어떻게
highlight/mask될지 미리 보는 local endpoint를 필요로 합니다. Service는 dictionary를
static하고 deterministic하게 유지합니다.

1. `POST /text/search-mask`는 text payload 하나와 optional mask rune을 받습니다.
2. Handler는 JSON을 검증하고 `Service.SearchMask`에 위임합니다.
3. Service는 NFC normalization과 Unicode word-boundary filtering으로 configured
   phrase를 검색합니다.
4. Response는 byte span, matched policy ID, masked projection, UI client용 Unicode
   caveat를 반환합니다.

## Architecture

![Gin text search service architecture](../../docs/images/readme-diagrams/gin-text-search-service-architecture.png)

| Layer | Owns | Does not own |
| --- | --- | --- |
| Gin handler | JSON binding, route shape, HTTP status, public error response. | Dictionary compile, matching, masking, tokenizer 선택. |
| `Service` | Policy loading, exact search, leftmost-longest overlap 처리, exact-span masking. | HTTP routing, auth, persistence, external moderation provider. |
| `textsearch` | Immutable matcher, NFC normalization, Unicode boundary check, original byte span. | Product policy, language detection, language-specific tokenization. |

## Processing Sequence

![Gin text search service sequence](../../docs/images/readme-diagrams/gin-text-search-service-sequence.png)

Endpoint는 tokenization이 아니라 exact search를 의도적으로 노출합니다. 예를 들어
`BoundaryUnicodeWord`가 켜져 있으면 `refund`는 `refundable` 내부에서 match되지
않고, `sale`은 `presale` 내부에서 match되지 않습니다. Offset은 original UTF-8
string의 byte span이므로 UI client는 display column 기준으로 자르기 전에 주의해서
mapping해야 합니다.

## Run

No-server preview를 출력합니다.

```bash
go run ./examples/gin-text-search-service
```

Gin service를 실행합니다.

```bash
SERVE_HTTP=1 go run ./examples/gin-text-search-service
```

기본 listen 주소는 `127.0.0.1:8098`입니다. `HTTP_ADDR=127.0.0.1:8099`로 바꿀 수
있습니다.

## Try The API

English/Korean mixed text를 search/mask합니다.

```bash
curl -s -X POST http://127.0.0.1:8098/text/search-mask \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"search-1001","text":"sale 쿠폰 refundable refund window bad wolf"}' | jq
```

Custom mask rune을 사용합니다.

```bash
curl -s -X POST http://127.0.0.1:8098/text/search-mask \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"search-1002","text":"presale refund refundable 쿠폰","mask":"#"}' | jq
```

Stable validation error를 확인합니다.

```bash
curl -s -X POST http://127.0.0.1:8098/text/search-mask \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"","text":"sale"}' | jq
```

Expected error code: `invalid_request`.

## Endpoint

| Method | Path | Success | Purpose |
| --- | --- | --- | --- |
| `GET` | `/healthz` | `200` | Process liveness only. |
| `POST` | `/text/search-mask` | `200` | Configured phrase를 검색하고 masked projection을 반환합니다. |

## What The Tests Prove

Focused test를 실행합니다.

```bash
go test -count=1 ./examples/gin-text-search-service/...
go test -race -count=1 ./examples/gin-text-search-service/...
```

Test는 다음을 증명합니다.

- Gin response shape는 stable하고 handler는 얇게 유지됩니다.
- Domain search는 Korean text, overlapping phrase, Unicode word boundary를
  처리합니다.
- Custom mask rune은 exact original match span에 적용됩니다.
- Public validation error는 stable response code를 사용합니다.
- Preview는 curl example과 Unicode caveat를 문서화합니다.

## Unicode Caveats

- `BoundaryUnicodeWord`는 substring false positive를 줄이는 helper이며 security
  boundary가 아닙니다.
- Response offset은 original UTF-8 string의 byte span이지 rune index나 display
  column이 아닙니다.
- NFC normalization은 켜져 있지만 language detection과 tokenizer 선택은 별도
  v0.8.0 lesson입니다.
- 이 예제에는 network, model, container dependency가 없습니다.
