# Gin 콘텐츠 검수 워크플로

[English](README.md)

이 실행형 예제는 [#53](https://github.com/debop/bluetape-go-workshop/issues/53), [#54](https://github.com/debop/bluetape-go-workshop/issues/54), [#55](https://github.com/debop/bluetape-go-workshop/issues/55), [#118](https://github.com/debop/bluetape-go-workshop/issues/118), [#119](https://github.com/debop/bluetape-go-workshop/issues/119)의 내용을 하나의 Gin API로 조합합니다. 서비스는 language detector, Japanese Search-mode tokenizer, simple tokenizer, blockword dictionary, bounded in-memory store를 한 번 생성해 소유합니다. Gin은 strict JSON, deadline, public error를 맡고 `main`은 HTTP lifecycle을 관리합니다.

## 실행

```bash
go run ./examples/gin-content-moderation-workflow
```

기본 주소는 `127.0.0.1:8080`입니다. `HTTP_ADDR`로 다른 loopback 주소를 지정할 수 있습니다. 원격 bind는 `ALLOW_UNAUTHENTICATED_REMOTE=1` 없이는 거부됩니다. 이 API에는 인증이 없고 create/get 응답이 원문을 노출하므로 원격 허용은 의도적으로 위험한 opt-in입니다.

```bash
curl -s http://127.0.0.1:8080/healthz
curl -s -X POST http://127.0.0.1:8080/moderation/records -H 'Content-Type: application/json' -d '{"content_id":"article-100","content":"배송 욕설 문의입니다","metadata":{"tenant":"demo"}}'
curl -s http://127.0.0.1:8080/moderation/records/article-100
curl -s -X POST http://127.0.0.1:8080/moderation/records/search -H 'Content-Type: application/json' -d '{"query":"배송 문의","metadata":{"tenant":"demo"},"limit":20,"after_content_id":"article-000"}'
```

신뢰도가 충분한 English, Korean, Japanese 콘텐츠는 `allowed` 또는 `masked`가 됩니다. 짧거나 unknown, low-confidence, mixed, Chinese, Kana 근거가 없는 Han-only 콘텐츠는 `[pending manual review]`를 가진 `manual-review`가 되며 검색에서 제외됩니다. 언어 판별은 heuristic이고 masking은 학습용 정책일 뿐 security, safety, compliance 경계가 아닙니다. Finding과 Japanese token span은 변경하지 않은 원문의 UTF-8 byte offset입니다.

검색은 URI 길이 제한을 피하고 metadata filter를 확장할 수 있도록 POST JSON을 사용합니다. 모든 query term과 metadata key/value가 정확히 일치해야 합니다. Metadata는 호출자가 소유하는 exact-match 데이터이며 create/get/search 응답에 나타나므로 credential이나 secret을 넣으면 안 됩니다. 검색 결과는 원문, finding, prepared term을 제외합니다. `truncated`가 true이면 `next_after_content_id`를 다음 exclusive cursor로 사용합니다.

## 제한과 오류

Body는 64 KiB, content/query는 8,000 rune, workflow 호출은 2초, store는 1,000 record로 제한됩니다. 검색은 기본 20개, 최대 100개입니다. 안정적인 오류 코드는 `invalid_request`, `unsupported_media_type`, `unsupported_content_encoding`, `request_too_large`, `request_timeout`, `duplicate_content_id`, `record_not_found`, `store_capacity_reached`, `internal_error`입니다.

Store는 프로세스 메모리에만 있어 재시작하면 사라집니다. 검색은 snapshot isolation이 없는 bounded linear scan입니다. 실제 서비스라면 인증, 권한, 보존 정책, durable storage, audit 정책, 애플리케이션 redaction을 추가해야 합니다.

## 검증

```bash
go test -count=1 ./examples/gin-content-moderation-workflow/...
go test -race -count=1 ./examples/gin-content-moderation-workflow/...
make ci
```
