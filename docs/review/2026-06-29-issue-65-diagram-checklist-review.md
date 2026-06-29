# Review: issue #65 diagram checklist repair

## Scope

- `docs/images/readme-diagrams/gin-sql-order-service-architecture.svg`
- `docs/images/readme-diagrams/gin-sql-order-service-sequence.svg`
- `docs/images/readme-diagrams/gin-sql-order-service-rollback.svg`
- rendered PNG siblings for the same diagrams

## Findings

P0=0 P1=0

## Checklist Evidence

| Gate | Status | Evidence |
| --- | --- | --- |
| Source-backed diagram semantics | PASS | Re-read `examples/gin-sql-order-service/internal/orderservice/service.go`; `CreateOrder` validates, creates IDs/time, enters `sqlkit.WithTx`, writes order rows, writes item rows, optionally returns `ErrOrderRejected`, appends status event, then reads the aggregate. |
| Best-practices visual family | PASS | Architecture is a static ownership map. Request and rollback images use the local sequence style: participant headers, lifelines, horizontal message lanes, subdued branch region, separated return lanes, and reader-facing labels. |
| XML parse | PASS | `xmllint --noout docs/images/readme-diagrams/gin-sql-order-service-{architecture,sequence,rollback}.svg`. |
| PNG render | PASS | `~/.local/bin/cairosvg <svg> -o <png> -s 2` for all three touched SVG files. |
| Full-size PNG inspection | PASS | Opened all three rendered PNGs after the final render and checked clipping, label overlap, arrowheads, branch text, and source direction. |
| Contact sheet | PASS | Created `/tmp/gin-sql-order-service-diagram-contact-sheet.png` with ImageMagick and inspected the architecture/sequence/rollback family together for drift. |
| Marker audit | PASS | CSS `marker-end` refs resolve to defined markers; all markers use `markerUnits="userSpaceOnUse"`; dashed return lines use solid grey marker heads. |
| Icon audit | PASS | Each diagram uses one catalog PostgreSQL icon with `data-bluetape4k-icon="testcontainers.postgresql"` and no duplicate `<use>`, legacy cylinder, or second database icon. |
| Helper-script availability | GAP | Current local `bluetape4k-diagram` skill bundle has no `references/diagram-geometry-audit.py` or `references/diagram-endpoint-audit.py`; used XML parse, CairoSVG render, marker/icon grep, contact sheet, and full-size PNG inspection as the equivalent gate. |

## Residual Risk

The changes are documentation-image only. No Go behavior changed.

