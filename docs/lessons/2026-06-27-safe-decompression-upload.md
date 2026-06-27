# Safe decompression upload example

Issue: #114

## Decision

Use `compression.DecompressLimit` in a runnable Gin upload API that accepts raw
`gzip` or `zstd` bytes and validates strict JSON only after both compressed and
expanded byte limits pass.

## Why

The existing cache snapshot example demonstrates compression tradeoffs for
trusted internal data. This example covers the untrusted HTTP boundary: a small
compressed request can expand into a large in-memory payload. The router owns
`http.MaxBytesReader` for compressed transport bytes, while the service owns
`compression.DecompressLimit` and typed error preservation for expanded bytes.

## Verification shape

- Service tests assert valid gzip and zstd payloads, malformed compressed bytes,
  unsupported algorithms, invalid expanded JSON, cancellation, and
  `errors.Is` preservation of `compression.ErrDecompressedSizeExceeded`.
- Router tests assert stable status/error-code mapping for accepted uploads,
  malformed compressed payloads, compressed request overflow, decompressed
  payload overflow, unsupported algorithms, canceled request contexts, health,
  and limit inspection.
- README diagrams show the boundary architecture, request sequence, and error
  policy separately so readers can see why compressed and decompressed limits
  protect different resources.
