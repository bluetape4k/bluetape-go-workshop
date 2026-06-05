# cache-snapshot-codecs

[English](README.md) | [한국어](README.ko.md)

Versioned cache snapshot example that combines `serialization` and
`compression`.

The example writes product catalog snapshots as JSON inside a versioned
serialization envelope, then compresses the payload with either `gzip` for broad
compatibility or `zstd` for fast internal storage. The tests include a size
comparison, but real algorithm choices still require workload-specific
measurement.

## Scenario

![Data codec and cleanup flow](../../docs/images/readme-diagrams/data-codec-cleanup-flow.png)

Use this example when a cache needs a portable snapshot format and a compact
payload. The versioned serializer keeps the payload shape explicit, while the
compressor remains a replaceable decision that can be measured independently.

## What It Demonstrates

- `serialization.VersionedSerializer` with JSON payloads.
- `compression.Gzip()` for broad compatibility.
- `compression.Zstd()` for fast internal storage.
- Stream decompression for larger snapshots.
- Corrupt compressed bytes and corrupt serialized payloads as separate failure
  cases.

## Run

```bash
go test -count=1 ./examples/cache-snapshot-codecs/...
```
