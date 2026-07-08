# Issue #54 Gin Text Search Service Code Review

Scope: `examples/gin-text-search-service`, root README catalog entries, README
diagram assets, and the #54 lessons/review artifacts.

Baseline: local branch `feat/issue-54-gin-text-search` against `origin/develop`.

## Findings

P0=0 P1=0

No blocking findings remain.

## Evidence

- `NewServer` wires only `/healthz` and `POST /text/search-mask`; the handler
  binds JSON, maps errors, and delegates search behavior to `Service.SearchMask`:
  `examples/gin-text-search-service/internal/searchapi/service.go:183`.
- `Service.SearchMask` validates request shape, enforces a one-rune mask,
  searches the compiled `textsearch.Matcher`, masks exact original spans, and
  returns summary counts plus Unicode caveats:
  `examples/gin-text-search-service/internal/searchapi/service.go:137`.
- Domain tests cover Korean text, leftmost-longest overlap, Unicode boundary
  behavior, custom mask output, and byte-span evidence:
  `examples/gin-text-search-service/internal/searchapi/service_test.go:13`.
- HTTP tests cover success response shape and stable validation errors:
  `examples/gin-text-search-service/internal/searchapi/service_test.go:43`.
- README files include curl examples, endpoint contract, Unicode caveats,
  architecture/sequence diagrams, and focused test commands:
  `examples/gin-text-search-service/README.md:8`.

## Validation

- `go test -count=1 ./examples/gin-text-search-service/...`
- `go test -race -count=1 ./examples/gin-text-search-service/...`
- `go run ./examples/gin-text-search-service`
- Temporary loopback server check with `SERVE_HTTP=1 HTTP_ADDR=127.0.0.1:18098 go run ./examples/gin-text-search-service`, verifying HTTP 200 success JSON and HTTP 400 `invalid_request`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `make ci`
- `xmllint --noout docs/images/readme-diagrams/gin-text-search-service-architecture.svg docs/images/readme-diagrams/gin-text-search-service-sequence.svg`
- `/Users/debop/.local/bin/cairosvg ... -s 2` for both SVG diagrams
- Full-size PNG inspection for both diagrams
- `git diff --check`

## Validation Gaps

No Testcontainers run is required for this issue. The example has no database,
queue, model, or external service dependency; HTTP behavior is covered with
`httptest` and domain behavior is covered with deterministic unit tests.

## Post-Merge Diagram Re-Audit

Scope: `docs/images/readme-diagrams/gin-text-search-service-architecture.*`
and `docs/images/readme-diagrams/gin-text-search-service-sequence.*`.

Findings fixed:

- SVG marker shapes now use fixed `userSpaceOnUse` filled-triangle paths with
  `stroke="none"` and `stroke-dasharray="none"` so CairoSVG PNG output keeps
  arrowhead direction and shape stable.
- Sequence message labels now sit 10px above their call/return lines instead
  of touching the line.
- Sequence participant, header, label, and message classes now match the local
  sequence-style audit contract.

Validation:

- `xmllint --noout docs/images/readme-diagrams/gin-text-search-service-architecture.svg docs/images/readme-diagrams/gin-text-search-service-sequence.svg`
- `cairosvg docs/images/readme-diagrams/gin-text-search-service-architecture.svg -o docs/images/readme-diagrams/gin-text-search-service-architecture.png`
- `cairosvg docs/images/readme-diagrams/gin-text-search-service-sequence.svg -o docs/images/readme-diagrams/gin-text-search-service-sequence.png`
- `diagram-connector-audit.py`: PASS for both SVGs
- `diagram-geometry-audit.py --fail-diagonal`: `geometry_failures=0` for both SVGs
- `diagram-endpoint-audit.py`: PASS for both SVGs
- `diagram-mixed-corner-audit.py`: PASS for both SVGs
- `diagram-sequence-style-audit.py`: PASS for the sequence SVG
- Custom label-gap check: `labels=8 min_gap=10.0px failures=0`
- Full-size PNG inspection for both regenerated PNG files
- `git diff --check`

### Follow-Up Boundary Arrow Correction

The PR #158 audit still missed the rendered direction/readability problem on
the `Boundary -> Original byte spans` connector. Root cause: the previous route
forced a dogleg with a terminal curve because the `Original byte spans` card
ended 5px left of the `Boundary` centerline. CairoSVG rendered a technically
valid marker, but the PNG did not read as a clean downward relationship.

Correction:

- Widened `Original byte spans` from 335px to 365px so the `Boundary` centerline
  lands inside the target card's top edge.
- Replaced the dogleg route with a single vertical helper connector:
  `M 1535 730 V 785`.
- Verified the rendered PNG crop and full-size PNG show a clear downward
  arrowhead from `Boundary` to `Original byte spans`.

Validation:

- `xmllint --noout docs/images/readme-diagrams/gin-text-search-service-architecture.svg`
- `cairosvg docs/images/readme-diagrams/gin-text-search-service-architecture.svg -o docs/images/readme-diagrams/gin-text-search-service-architecture.png -s 2`
- `diagram-connector-audit.py`: PASS
- `diagram-geometry-audit.py --fail-diagonal`: `geometry_failures=0`
- `diagram-endpoint-audit.py`: PASS
- `diagram-mixed-corner-audit.py`: PASS
- Custom invariant: `path=vertical_down`, `target_inside_top_guard=True`, `card_width=365`
- Full-size PNG inspection plus focused crop inspection for the boundary arrow
- `git diff --check`
