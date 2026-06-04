# cache-snapshot-codecs

Versioned cache snapshot example that combines `serialization` and
`compression`.

The example writes product catalog snapshots as JSON inside a versioned
serialization envelope, then compresses the payload with either `gzip` for broad
compatibility or `zstd` for fast internal storage. The tests include a size
comparison, but real algorithm choices still require workload-specific
measurement.
