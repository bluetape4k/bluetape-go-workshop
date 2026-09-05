# Graph Import/Export

[English](README.md) | [한국어](README.ko.md)

This `v0.10.0` workshop example implements issue #52 as a small risk-analysis
import job for a named partner. It uses the released `graph/graphio` record
boundary: NDJSON is the default interchange, and the optional bounded
`graph/graphio/graphml` package handles an explicitly selected GraphML subset.
The application validates the domain graph, preserves scalar values and edge
direction, and publishes a deterministic summary plus normalized snapshot.

## Package lesson

The example keeps one narrow application boundary:

- `Account` and `Device` vertices carry a `partner` property. Account records
  may include integer risk scores and boolean state; device records include a
  kind and trust flag.
- `USES_DEVICE` edges are directed from an `Account` to a `Device` and carry a
  floating-point confidence value.
- Record IDs are unique across vertices and edges. Vertices and edges are
  normalized into ascending ID order before export and snapshot publication.
- JSON integers in the safe range are normalized to `int64`; GraphML typed
  scalar values are restored as `bool`, `int64`, `float64`, or `string` (a
  declared `double` remains `float64` even when its value is integral).
  Composite values, nulls, and non-finite numbers are rejected.

The fixed `risk-partner-acme-v1` fixture has four vertices and three directed
edges. `RunDemo` exports and imports it through both formats, then compares the
two normalized snapshots.

## Bounds and fail-closed behavior

`DefaultOptions` keeps untrusted input bounded:

| Boundary | Default |
| --- | ---: |
| Whole input | `64 KiB` |
| NDJSON line | `16 KiB` |
| NDJSON record | `16 KiB` |
| Total records | `64` |

The job fails closed for duplicate IDs, missing endpoints, wrong partner
properties, unsupported labels or edge direction, non-scalar properties,
unknown GraphML keys, XML directives/extensions, nested graphs, and oversized
input. The GraphML reader is a whole-document bounded parser; a caller that
owns a blocking reader must still close it or provide a deadline to unblock I/O
after cancellation.

## Run

Print the deterministic comparison report:

```bash
go run ./examples/graph-import-export
```

The report contains `partner`, `equivalent`, one run for `ndjson` and one for
`graphml`, per-format counts, and the ID-ordered normalized snapshot. The
stable summary starts like this:

```json
{
  "partner": "acme-payments",
  "equivalent": true,
  "runs": [
    {
      "format": "ndjson",
      "summary": {
        "vertices": 4,
        "edges": 3,
        "accounts": 2,
        "devices": 2,
        "uses_device_edges": 3,
        "directed": true
      }
    },
    {
      "format": "graphml",
      "summary": {
        "vertices": 4,
        "edges": 3,
        "accounts": 2,
        "devices": 2,
        "uses_device_edges": 3,
        "directed": true
      }
    }
  ]
}
```

The normalized snapshot keeps the same order and directed endpoints in both
runs (excerpt):

```json
{
  "vertices": [
    {"id": "account-a", "label": "Account"},
    {"id": "account-b", "label": "Account"},
    {"id": "device-01", "label": "Device"},
    {"id": "device-02", "label": "Device"}
  ],
  "edges": [
    {"id": "use-001", "from": "account-a", "to": "device-01"}
  ]
}
```

Run the focused and race tests:

```bash
go test -count=1 ./examples/graph-import-export/...
go test -race -count=1 ./examples/graph-import-export/...
```

## Supported boundary and production scope

The lesson demonstrates NDJSON round-trip, bounded GraphML round-trip,
deterministic ordering, scalar normalization, and inspectable failure classes.
Paired CSV remains available from `graph/graphio`, but this CLI deliberately
keeps the comparison to the default NDJSON path and the named partner's
GraphML path.

It does not claim broad GraphML, yEd, yFiles, Gephi, NetworkX, or Neo4j APOC
compatibility. Compression, encryption, filesystem ownership, atomic file
replacement, graph database adapters, schema migration, and production risk
decisions remain caller-owned and outside this workshop example.
