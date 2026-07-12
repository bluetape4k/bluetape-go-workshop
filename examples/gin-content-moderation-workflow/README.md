# Gin content moderation workflow

[한국어](README.ko.md)

This runnable example turns the lessons from [#53](https://github.com/debop/bluetape-go-workshop/issues/53), [#54](https://github.com/debop/bluetape-go-workshop/issues/54), [#55](https://github.com/debop/bluetape-go-workshop/issues/55), [#118](https://github.com/debop/bluetape-go-workshop/issues/118), and [#119](https://github.com/debop/bluetape-go-workshop/issues/119) into one application-shaped Gin API. One service owns the shared language detector, Japanese Search-mode tokenizer, simple tokenizer, blockword dictionary, and bounded in-memory store; Gin owns strict JSON, deadlines, and public errors; `main` owns the HTTP lifecycle.

## Run

```bash
go run ./examples/gin-content-moderation-workflow
```

The default address is `127.0.0.1:8080`. `HTTP_ADDR` may select another loopback address. A remote bind is rejected unless `ALLOW_UNAUTHENTICATED_REMOTE=1`; this API has no authentication and exposes original content from create/get, so remote opt-in is intentionally unsafe.

```bash
curl -s http://127.0.0.1:8080/healthz
curl -s -X POST http://127.0.0.1:8080/moderation/records -H 'Content-Type: application/json' -d '{"content_id":"article-100","content":"배송 욕설 문의입니다","metadata":{"tenant":"demo"}}'
curl -s http://127.0.0.1:8080/moderation/records/article-100
curl -s -X POST http://127.0.0.1:8080/moderation/records/search -H 'Content-Type: application/json' -d '{"query":"배송 문의","metadata":{"tenant":"demo"},"limit":20,"after_content_id":"article-000"}'
```

Supported confident English, Korean, and Japanese content becomes `allowed` or `masked`. Short, unknown, low-confidence, mixed, Chinese, or ambiguous Han-only content becomes `manual-review` with `[pending manual review]` and is excluded from search. Detection is heuristic, and masking is a teaching policy—not a security, safety, or compliance boundary. Finding and Japanese-token spans are UTF-8 byte offsets into the unchanged original content.

Search uses POST JSON to avoid URI length limits and allow metadata filters. Every query term and metadata key/value must match; metadata is caller-owned exact-match data returned by create/get/search. Never put credentials or secrets in it. Results omit original content, findings, and prepared terms. Use `next_after_content_id` as the next exclusive cursor when `truncated` is true.

## Bounds and errors

Bodies are limited to 64 KiB, content/query to 8,000 runes, workflow calls to 2 seconds, storage to 1,000 records, and search pages to 20 by default or 100 maximum. Stable codes are `invalid_request`, `unsupported_media_type`, `unsupported_content_encoding`, `request_too_large`, `request_timeout`, `duplicate_content_id`, `record_not_found`, `store_capacity_reached`, and `internal_error`.

The store is process-local and lost on restart. Search is a bounded linear scan with no snapshot isolation. A production service must add authentication, authorization, retention, durable storage, audit policy, and application-owned redaction.

## Validate

```bash
go test -count=1 ./examples/gin-content-moderation-workflow/...
go test -race -count=1 ./examples/gin-content-moderation-workflow/...
make ci
```
