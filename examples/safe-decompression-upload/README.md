# safe-decompression-upload

[English](README.md) | [한국어](README.ko.md)

Gin upload ingestion example for `compression.DecompressLimit`.

This example treats compressed HTTP request bodies as untrusted input. It applies
two independent limits: the HTTP layer bounds the compressed bytes with
`http.MaxBytesReader`, and the service layer bounds the expanded payload with
`compression.DecompressLimit`. The goal is to show how an application can accept
small compressed uploads without letting a tiny request expand into an oversized
in-memory payload.

## Scenario

An ingestion API accepts JSON documents from partners. Partners may send `gzip`
or `zstd` bytes to save bandwidth, but compression changes the risk profile: a
small compressed body can expand into a much larger document. The service
therefore has to defend two boundaries:

- the transport boundary, where the compressed request body is read from HTTP;
- the decompression boundary, where trusted application code first sees expanded
  bytes.

The example accepts only a strict JSON document after both boundaries pass. It
returns the algorithm, compressed size, decompressed size, and SHA-256 of the
expanded payload, but it still marks `authoritative_scan_still_needed=true`.
Decompression is not authorization, malware scanning, or schema governance.

## Architecture

![Safe decompression upload architecture](../../docs/images/readme-diagrams/safe-decompression-upload-architecture.png)

The HTTP layer owns compressed request-size defense. The service layer owns
decompression and JSON validation. Keeping those responsibilities separate makes
each failure precise: a large compressed body maps to
`compressed_request_too_large`, while a small body that expands beyond the
configured limit maps to `decompressed_payload_too_large`.

## Request Flow

![Safe decompression upload sequence](../../docs/images/readme-diagrams/safe-decompression-upload-sequence.png)

1. The client posts raw compressed bytes to `/uploads/compressed`.
2. The router wraps the body with `http.MaxBytesReader` before calling
   `io.ReadAll`.
3. The service selects `gzip` or `zstd` from `X-Compression-Algorithm` or
   `Content-Encoding`.
4. `compression.DecompressLimit` reads at most `maxBytes+1` expanded bytes and
   returns `compression.ErrDecompressedSizeExceeded` when the limit is crossed.
5. The expanded bytes are decoded with unknown-field rejection and required
   `document_id`, `tenant`, and `body` fields.

## Error Policy

![Safe decompression upload policy](../../docs/images/readme-diagrams/safe-decompression-upload-policy.png)

| Boundary | Status | Error code |
|---|---|---|
| Compressed request exceeds `MaxBytesReader` | `413` | `compressed_request_too_large` |
| Expanded payload exceeds `DecompressLimit` | `413` | `decompressed_payload_too_large` |
| Bad gzip/zstd bytes | `400` | `malformed_compressed_payload` |
| Missing or unsupported compression algorithm | `400` | `unsupported_compression` |
| Invalid expanded JSON | `400` | `invalid_upload_payload` |
| Request context canceled before ingestion completes | `408` | `request_cancelled` |

The service wraps `compression.ErrDecompressedSizeExceeded` so callers can still
use `errors.Is(err, compression.ErrDecompressedSizeExceeded)` in tests or higher
layers while presenting a stable public HTTP error.

## What It Demonstrates

- `compression.DecompressLimit` for untrusted compressed upload bytes.
- Typed error preservation with `errors.Is`.
- `http.MaxBytesReader` for compressed request-size defense.
- Strict JSON decoding after decompression.
- Stable HTTP status and public error code mapping.
- Valid payload, malformed compressed data, decompressed-size overflow,
  compressed request overflow, unsupported algorithm, invalid payload, and
  cancellation tests.

## Run

Run the local upload guard API:

```bash
go run ./examples/safe-decompression-upload
```

The service listens on `127.0.0.1:8103` by default.

```bash
curl http://127.0.0.1:8103/healthz
curl http://127.0.0.1:8103/uploads/limits
```

Create a small gzip upload and send it as raw bytes:

```bash
printf '%s' '{"document_id":"doc-1001","tenant":"checkout","body":"invoice upload"}' \
  | gzip -c \
  | curl -s -X POST http://127.0.0.1:8103/uploads/compressed \
      -H 'X-Compression-Algorithm: gzip' \
      --data-binary @- | jq
```

Expected response: `202 Accepted`, `decision=accepted`, plus compressed and
decompressed byte counts.

Useful environment variables:

| Variable | Default | Purpose |
|---|---|---|
| `HTTP_ADDR` | `127.0.0.1:8103` | Loopback HTTP bind address. Non-loopback binds are rejected. |
| `COMPRESSED_BODY_LIMIT_BYTES` | `32768` | Maximum bytes read from the compressed HTTP request body. |
| `DECOMPRESSED_BODY_LIMIT_BYTES` | `262144` | Maximum bytes retained after decompression. |

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Process liveness only. |
| `GET` | `/uploads/limits` | Inspect compressed and decompressed upload limits. |
| `POST` | `/uploads/compressed` | Upload raw `gzip` or `zstd` bytes for bounded decompression. |

## Boundary Notes

- Always bound compressed bytes before reading the HTTP body into memory.
- Always bound expanded bytes before trusting decompressed payloads.
- A successful decompression only proves the bytes fit and decoded as expected.
  It is not authorization, content safety, malware scanning, or durable
  ingestion.
- Keep public errors stable. Decoder internals should remain server-side
  diagnostics.
- Tune compressed and decompressed limits separately; they protect different
  resources.

## Test

```bash
go test -count=1 ./examples/safe-decompression-upload/...
go test -race -count=1 ./examples/safe-decompression-upload/...
```
