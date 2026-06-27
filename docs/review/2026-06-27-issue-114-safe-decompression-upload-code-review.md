# Code review: issue #114 safe decompression upload

## Scope

- New runnable example: `examples/safe-decompression-upload`
- New README diagrams for architecture, request sequence, and error policy
- Root README navigation and focused run instructions
- Lesson note for untrusted compressed upload handling

## Findings

No P0/P1 findings in the local review pass.

## Checks

- Compressed request bodies are bounded with `http.MaxBytesReader` before
  `io.ReadAll`.
- Expanded payloads are bounded with `compression.DecompressLimit`.
- `compression.ErrDecompressedSizeExceeded` remains observable through
  `errors.Is`.
- Public HTTP errors distinguish compressed request overflow, decompressed
  payload overflow, malformed compressed bytes, unsupported algorithms, invalid
  expanded JSON, and cancellation.
- The runtime HTTP server binds to loopback by default and rejects non-loopback
  `HTTP_ADDR` values.
- README diagrams render as PNG and keep boundary, flow, and policy concerns
  separated.

## Residual risk

The example intentionally stops at bounded decompression and strict JSON
validation. Malware scanning, authorization, durable storage, and deeper schema
governance are documented as downstream responsibilities.
