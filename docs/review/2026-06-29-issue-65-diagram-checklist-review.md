# Review: issue #65 diagram checklist repair

## 범위

- `docs/images/readme-diagrams/gin-sql-order-service-architecture.svg`
- `docs/images/readme-diagrams/gin-sql-order-service-sequence.svg`
- `docs/images/readme-diagrams/gin-sql-order-service-rollback.svg`
- 같은 diagram의 rendered PNG sibling

## 발견 사항

P0=0 P1=0

## Checklist Evidence

| Gate | Status | Evidence |
| --- | --- | --- |
| Source-backed diagram semantics | PASS | `examples/gin-sql-order-service/internal/orderservice/service.go`를 다시 읽었다. `CreateOrder`는 validate, ID/time 생성, `sqlkit.WithTx` 진입, order row write, item row write, optional `ErrOrderRejected`, status event append, aggregate read 순서로 동작한다. |
| Best-practices visual family | PASS | architecture는 static ownership map이다. request/rollback image는 participant header, lifeline, horizontal message lane, subdued branch region, separated return lane, reader-facing label을 갖춘 local sequence style을 사용한다. |
| XML parse | PASS | `xmllint --noout docs/images/readme-diagrams/gin-sql-order-service-{architecture,sequence,rollback}.svg`. |
| PNG render | PASS | 변경한 SVG 3개 모두에 `~/.local/bin/cairosvg <svg> -o <png> -s 2`를 실행했다. |
| Full-size PNG inspection | PASS | final render 이후 rendered PNG 3개를 모두 열어 clipping, label overlap, arrowhead, branch text, source direction을 확인했다. |
| Contact sheet | PASS | ImageMagick으로 `/tmp/gin-sql-order-service-diagram-contact-sheet.png`를 만들고 architecture/sequence/rollback family를 함께 inspect해 drift를 확인했다. |
| Marker audit | PASS | CSS `marker-end` reference는 defined marker로 resolve된다. 모든 marker는 `markerUnits="userSpaceOnUse"`를 사용하고 dashed return line은 solid grey marker head를 사용한다. |
| Icon audit | PASS | 각 diagram은 `data-bluetape4k-icon="testcontainers.postgresql"`가 있는 catalog PostgreSQL icon 하나만 사용하며 duplicate `<use>`, legacy cylinder, second database icon은 없다. |
| Helper-script availability | GAP | 현재 local `bluetape4k-diagram` skill bundle에는 `references/diagram-geometry-audit.py`나 `references/diagram-endpoint-audit.py`가 없다. equivalent gate로 XML parse, CairoSVG render, marker/icon grep, contact sheet, full-size PNG inspection을 사용했다. |

## 잔여 Risk

변경은 documentation image 전용이다. Go behavior는 바뀌지 않았다.
