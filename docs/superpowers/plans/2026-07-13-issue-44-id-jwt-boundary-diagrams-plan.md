# Issue #44 ID/JWT Boundary Diagrams Implementation Plan

> **agentic worker 대상:** REQUIRED SUB-SKILL: 이 계획은 task 단위로 구현한다. `superpowers:subagent-driven-development` 사용을 권장하며, 대안으로 `superpowers:executing-plans`를 사용할 수 있다. 진행 추적은 checkbox (`- [ ]`) syntax를 사용한다.

**Goal:** ID/JWT demo scenario, runtime trust ownership, protected order request sequence를 설명하는 source-backed bilingual README diagram 세 개를 추가한다.

**Architecture:** documentation-only 변경이다. 각 diagram은 canonical README asset directory에 hand-authored SVG로 작성하고, CairoSVG로 paired 2x PNG를 렌더링한다. 각 asset은 독립 audit, full/original detail visual inspection을 거쳐 두 README locale에 같은 순서로 embed한다.

**Tech Stack:** SVG, CairoSVG, XMLLint, bluetape-diagram audit script, ImageMagick metadata inspection, Markdown, Go repository verification를 사용한다.

---

## File Map

- Create: `docs/images/readme-diagrams/id-jwt-boundary-scenario.svg`
- Create: `docs/images/readme-diagrams/id-jwt-boundary-scenario.png`
- Create: `docs/images/readme-diagrams/id-jwt-boundary-architecture.svg`
- Create: `docs/images/readme-diagrams/id-jwt-boundary-architecture.png`
- Create: `docs/images/readme-diagrams/id-jwt-boundary-sequence.svg`
- Create: `docs/images/readme-diagrams/id-jwt-boundary-sequence.png`
- Modify: `examples/id-jwt-boundary/README.md`
- Modify: `examples/id-jwt-boundary/README.ko.md`
- Modify: `docs/lessons/2026-06-22-id-jwt-boundary.md`
- Modify: `docs/superpowers/specs/2026-07-13-issue-44-id-jwt-boundary-diagrams-design.md`
- Create: `docs/superpowers/plans/2026-07-13-issue-44-id-jwt-boundary-diagrams-plan.md`

root README file, Go source, test, module metadata, workflow는 scope 밖이다.

## Task 1: Source and Visual Evidence 고정

- [ ] **Step 1: source invariant 확인**

실행한다.

```bash
sed -n '1,220p' examples/id-jwt-boundary/README.md
sed -n '1,430p' examples/id-jwt-boundary/internal/idjwtboundary/service.go
sed -n '1,380p' examples/id-jwt-boundary/internal/idjwtboundary/service_test.go
```

기대값: bounded `/tokens` and `/orders` JSON, fixed-HMAC demo composition, expected issuer/audience/expiration parsing, `customer` role, `orders:create` scope, validation before two UUID v7 generations, stable public error mapping이 존재한다.

- [ ] **Step 2: 승인된 visual baseline 검사**

다음 PNG를 original detail로 연다.

```text
/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/workflow-image-upload.png
/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/utils-idgenerators-diagram-03.png
/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/leader-ktor-architecture-01.png
/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/sequence-workflow-sample.png
/Users/debop/work/bluetape4k/bluetape-go-workshop/.worktrees/docs-issue-44-id-jwt-boundary-diagrams/docs/images/readme-diagrams/gin-audit-query-api-sequence.png
```

Original-detail inspection ledger:

- `workflow-image-upload.png` — 1200x480; numbered primary stage가 왼쪽에서 오른쪽으로 읽히며, supporting responsibility card가 main path 아래에 놓인다.
- `utils-idgenerators-diagram-03.png` — 6000x3400; timestamp, machine, sequence, packing, result responsibility가 uniform grid가 아니라 semantic region을 이룬다.
- `leader-ktor-architecture-01.png` — 2640x1800; application code, Ktor process, integration layer, core API, backend implementation의 framework/ownership boundary가 명확하다.
- `sequence-workflow-sample.png` — 3720x2272; rectangular participant, dashed lifeline, activation, numbered message pill, visible call/return head, transparent retry frame이 catalog sequence family를 만든다.
- `gin-audit-query-api-sequence.png` — 3360x2520; 가장 가까운 repo-local Gin reference이며 같은 signal을 보존하면서 Gin adapter boundary, explicit success/error branch, sender-colored message를 추가한다.

기대값: scenario stage는 한 방향으로 읽힌다. architecture는 box grid 대신 responsibility region과 명시적인 framework/application/provider boundary를 사용한다. 두 sequence reference는 rectangular participant, dashed lifeline, activation bar, numbered label, visible arrowhead, transparent branch frame을 사용한다. repo-local Gin sequence가 가장 가까운 module-family baseline이다.

- [ ] **Step 3: marker geometry 고정**

다음 exact invariant를 사용한다.

```text
primary/scenario marker: markerWidth="14" markerHeight="14" viewBox="0 0 14 14"
primary/scenario reference: refX="12" refY="7"
primary/scenario triangle: d="M2 2 L12 7 L2 12 Z" with explicit fill
sequence marker: markerWidth="16" markerHeight="16" viewBox="0 0 16 16"
sequence reference: refX="13" refY="8"
sequence triangle: d="M3 3 L13 8 L3 13 Z" with explicit fill
all markers: markerUnits="userSpaceOnUse" orient="auto"
marker color: explicit marker per semantic connector color
primary and sequence terminal segment after final bend: >= 24 px
attachment: perpendicular to the touched card edge
endpoint corner clearance: >= max(8 px, rx / 2)
connector endpoint: exactly on the card boundary, never inside the card
```

기대값: final segment가 rendered arrow direction을 결정하며, 어떤 bend도 marker footprint를 침범하지 않는다.

## Task 2: Scenario Diagram 생성 및 증명

- [ ] **Step 1: scenario SVG 생성**

`docs/images/readme-diagrams/id-jwt-boundary-scenario.svg`를 1200x650 light canvas에 생성한다. rounded outer decorator, title, subtitle, 왼쪽에서 오른쪽으로 이어지는 numbered card 여섯 개, lower trust-notes band 하나를 사용한다.

```text
1 Request Demo Token
  POST /tokens · subject · role · scopes · ttl
2 Sign Claims
  HS256 · issuer=id-jwt-boundary · audience=orders-api
3 Submit Order
  POST /orders · Authorization: Bearer JWT
4 Verify Trust
  signature · issuer · audience · expiration
5 Authorize + Validate
  role=customer · orders:create · SKU · quantity
6 Create Internal IDs
  UUID v7 order_id + request_id · 201 Created
```

trust-notes band에는 다음 문구를 넣는다.

```text
Signed claims are not encrypted
UUIDs are identifiers, not credentials
The fixed HMAC secret is local demo material only
```

request progress에는 blue, JWT trust에는 violet, authorized ID creation에는 olive를 사용한다. 모든 progression line은 orthogonal이어야 하며, 필요한 corner clearance를 지키면서 card edge에 perpendicular로 붙고, final bend 뒤에는 최소 24 px를 남긴다.

- [ ] **Step 2: scenario 검증 및 렌더링**

실행한다.

```bash
xmllint --noout docs/images/readme-diagrams/id-jwt-boundary-scenario.svg
cairosvg docs/images/readme-diagrams/id-jwt-boundary-scenario.svg -o docs/images/readme-diagrams/id-jwt-boundary-scenario.png -s 2
identify docs/images/readme-diagrams/id-jwt-boundary-scenario.png
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-connector-audit.py" docs/images/readme-diagrams/id-jwt-boundary-scenario.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-geometry-audit.py" --fail-diagonal docs/images/readme-diagrams/id-jwt-boundary-scenario.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py" docs/images/readme-diagrams/id-jwt-boundary-scenario.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py" docs/images/readme-diagrams/id-jwt-boundary-scenario.svg
```

기대값: XML/render success, PNG 2400x1300, primary progression connector 다섯 개, diagonal/endpoint/mixed-corner failure 0개.

- [ ] **Step 3: rendered scenario 검사**

2400x1300 PNG를 full detail과 original detail로 연다. 다섯 endpoint를 모두 native pixel에서 검사한다. 모든 arrow가 다음 card를 향하고, head가 clipped/detached 되지 않으며, line이 card를 가로지르지 않고, label이 맞으며, trust band가 secondary로 남고, margin이 균형적인지 확인한다.

- [ ] **Step 4: scenario pair commit**

```bash
git add docs/images/readme-diagrams/id-jwt-boundary-scenario.svg docs/images/readme-diagrams/id-jwt-boundary-scenario.png
git commit -m "docs: add ID JWT boundary scenario diagram"
```

## Task 3: Architecture Diagram 생성 및 증명

- [ ] **Step 1: architecture SVG 생성**

`docs/images/readme-diagrams/id-jwt-boundary-architecture.svg`를 1400x900 light canvas에 생성한다. flat grid가 아니라 responsibility region과 aligned card를 사용한다.

```text
Caller and demo boundary
  Demo Client -> POST /tokens
  Order Client -> Bearer JWT + order JSON

HTTP runtime boundary
  main + net/http: loopback HTTP server + timeouts
  Gin Router: routing + Recovery + no trusted proxies
  route adapters: 8 KiB body limit + JSON/status mapping

Application trust boundary
  IssueToken: request validation + claim assembly
  CreateOrder: Bearer extraction + trust checks
  Local policy: customer + orders:create
  Order validation + allowlisted public errors

bluetape-go providers
  jwt.Provider from NewFixedHMACProvider(HS256, ...): compose/parse signed claims
  id.StringGenerator from NewUUIDV7Generator(): order_id and request_id after authorization

Public outcomes
  200 demo token
  201 accepted order
  400/401/403/500 stable error response
```

HTTP/application ownership에는 blue, signed-claim composition/verification에는 violet, authorized UUID path에는 olive, failure mapping에는 muted red를 사용한다. in-image legend를 포함한다. `/tokens`와 `/orders`는 shared service boundary에 도달하기 전까지 분리한다. UUID generation을 failure path에 연결하면 안 된다.

- [ ] **Step 2: architecture 검증 및 렌더링**

실행한다.

```bash
xmllint --noout docs/images/readme-diagrams/id-jwt-boundary-architecture.svg
cairosvg docs/images/readme-diagrams/id-jwt-boundary-architecture.svg -o docs/images/readme-diagrams/id-jwt-boundary-architecture.png -s 2
identify docs/images/readme-diagrams/id-jwt-boundary-architecture.png
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-connector-audit.py" docs/images/readme-diagrams/id-jwt-boundary-architecture.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-geometry-audit.py" --fail-diagonal docs/images/readme-diagrams/id-jwt-boundary-architecture.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py" docs/images/readme-diagrams/id-jwt-boundary-architecture.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py" docs/images/readme-diagrams/id-jwt-boundary-architecture.svg
```

기대값: XML/render success, PNG 2800x1800, card 최소 12개, connector 최소 10개, automated audit failure 0개.

- [ ] **Step 3: architecture connector geometry 검사**

final PNG를 full detail과 original detail로 연다. 모든 connector에서 SVG final segment와 rendered head를 비교하고 다음을 확인한다.

```text
arrowhead direction matches the final path segment
final segment is >= 24 px after the last bend
marker tip contacts the intended card boundary
attachment is perpendicular with corner clearance >= max(8 px, rx / 2)
bend/corner does not overlap the marker footprint
no connector crosses a card, label, lane title, or another line
no connector rides a card edge or appears tangent
```

failure가 있으면 port 또는 bend coordinate를 이동하고, 다시 render하며, 모든 architecture audit를 다시 실행한 뒤 full-size inspection을 반복한다.

- [ ] **Step 4: architecture pair commit**

```bash
git add docs/images/readme-diagrams/id-jwt-boundary-architecture.svg docs/images/readme-diagrams/id-jwt-boundary-architecture.png
git commit -m "docs: add ID JWT boundary architecture diagram"
```

## Task 4: Request Sequence 생성 및 증명

- [ ] **Step 1: sequence SVG 생성**

`docs/images/readme-diagrams/id-jwt-boundary-sequence.svg`를 1600x1300 light canvas에 생성하고 participant `Caller`, `Gin Router`, `Boundary Service`, `JWT Provider`, `UUID v7 Generator`를 배치한다. rectangular participant header, arrowhead 없는 dashed lifeline 다섯 개, activation bar, transparent branch frame, numbered message pill, continuous message line을 사용한다.

success order는 다음과 같다.

```text
1  POST /orders + Bearer JWT
2  parse bounded order JSON
3  CreateOrder(authorization, request)
4  extract Bearer token
5  Parse(expected issuer, audience, expiration)
6  verified subject + role + scope
7  require customer + orders:create
8  validate SKU + quantity
9  NextString() for order_id
10 NextString() for request_id
11 accepted order projection
12 201 Created
```

실제 decision level에 compact failure branch를 추가한다.

```text
missing token -> 401 missing_token before JWT parse
expired token -> 401 expired_token after JWT parse
other parse/signature/key failure -> 401 invalid_token
verified wrong role/scope -> 403 forbidden before UUID generation
invalid order -> 400 invalid_request before UUID generation
ID generation failure -> 500 internal_error without internals
```

solid call, dashed return, failure 전용 red, JWT message용 violet, UUID creation용 olive를 사용한다. 각 explicit per-color sequence marker는 locked 16x16 geometry, `markerUnits="userSpaceOnUse"`, `orient="auto"`, matching fill color, final bend 뒤 최소 24 px straight terminal route를 사용한다. message endpoint는 participant 또는 activation boundary에 perpendicular로 붙이고, rounded card와 관련된 곳은 최소 `max(8 px, rx / 2)` corner clearance를 지킨다.

- [ ] **Step 2: sequence 검증 및 렌더링**

실행한다.

```bash
xmllint --noout docs/images/readme-diagrams/id-jwt-boundary-sequence.svg
cairosvg docs/images/readme-diagrams/id-jwt-boundary-sequence.svg -o docs/images/readme-diagrams/id-jwt-boundary-sequence.png -s 2
identify docs/images/readme-diagrams/id-jwt-boundary-sequence.png
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-connector-audit.py" docs/images/readme-diagrams/id-jwt-boundary-sequence.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-geometry-audit.py" --fail-diagonal docs/images/readme-diagrams/id-jwt-boundary-sequence.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py" docs/images/readme-diagrams/id-jwt-boundary-sequence.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py" docs/images/readme-diagrams/id-jwt-boundary-sequence.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-sequence-style-audit.py" docs/images/readme-diagrams/id-jwt-boundary-sequence.svg
```

기대값: PNG 3200x2600, participant header/lifeline 다섯 개, numbered success message 12개, transparent nonempty failure branch, common failure 0개, sequence-style audit PASS.

- [ ] **Step 3: sequence direction 검사**

final PNG를 full detail과 original detail로 연다. solid call은 오른쪽을 향하고 dashed return은 왼쪽을 향하는지, label이 끊기지 않은 line 위에 놓이는지, activation bar가 marker head를 가리지 않는지, branch frame이 chronology를 보존하는지, failure branch가 UUID generation에 도달하지 않는지 확인한다.

- [ ] **Step 4: sequence pair commit**

```bash
git add docs/images/readme-diagrams/id-jwt-boundary-sequence.svg docs/images/readme-diagrams/id-jwt-boundary-sequence.png
git commit -m "docs: add ID JWT boundary sequence diagram"
```

## Task 5: README Locale 및 Lesson 통합

- [ ] **Step 1: English README 업데이트**

`## Scenario` 아래에 scenario PNG를 추가하고, local demo issuer와 protected order boundary를 분리한다는 점을 설명한다. `## What It Demonstrates` 앞에 `## Architecture`를 추가하고 architecture PNG와 ownership 설명을 넣는다. `## Boundary Notes` 앞에 `## Protected Order Sequence`를 추가하고 sequence PNG를 넣으며, auth failure는 UUID generation 전에 종료된다고 명시한다.

다음 link를 사용한다.

```markdown
![ID and JWT boundary scenario](../../docs/images/readme-diagrams/id-jwt-boundary-scenario.png)
![ID and JWT boundary architecture](../../docs/images/readme-diagrams/id-jwt-boundary-architecture.png)
![ID and JWT protected order sequence](../../docs/images/readme-diagrams/id-jwt-boundary-sequence.png)
```

- [ ] **Step 2: Korean README 업데이트**

corresponding heading 아래에 같은 path 세 개를 같은 순서로 추가한다. `/tokens`, `/orders`, `issuer`, `audience`, `expiration`, `customer`, `orders:create`, UUID v7, public error code는 보존하면서 자연스럽게 localize한다.

- [ ] **Step 3: lesson 업데이트**

`docs/lessons/2026-06-22-id-jwt-boundary.md`에 `## Diagram Evidence`를 추가하고 다음 point를 기록한다.

```text
The three assets are source-backed and shared by both README locales.
SVG-to-PNG review checks arrowhead direction, marker color parity, endpoint contact, and native-pixel readability.
Connector bends reserve straight terminal distance for the marker footprint; automated geometry success alone is insufficient.
JWT failure paths visibly end before UUID generation, so identifiers never appear to authorize a request.
```

- [ ] **Step 4: locale order 및 link 검증**

실행한다.

```bash
diff <(rg -o 'readme-diagrams/[^) ]+\.png' examples/id-jwt-boundary/README.md) <(rg -o 'readme-diagrams/[^) ]+\.png' examples/id-jwt-boundary/README.ko.md)
for asset in $(rg -o 'readme-diagrams/[^) ]+\.png' examples/id-jwt-boundary/README.md | sed 's#readme-diagrams/##'); do test -f "docs/images/readme-diagrams/$asset"; test -f "docs/images/readme-diagrams/${asset%.png}.svg"; done
git diff --check
```

기대값: locale diff가 비어 있고, 세 PNG/SVG pair가 모두 resolve되며, diff check가 exit zero로 끝난다.

- [ ] **Step 5: README 및 lesson integration commit**

```bash
git add examples/id-jwt-boundary/README.md examples/id-jwt-boundary/README.ko.md docs/lessons/2026-06-22-id-jwt-boundary.md
git commit -m "docs: explain ID JWT boundary diagrams"
```

## Task 6: Final Verification and PR Delivery

- [ ] **Step 1: 모든 final diagram gate 재실행**

실행한다.

```bash
for name in scenario architecture sequence; do
  svg="docs/images/readme-diagrams/id-jwt-boundary-${name}.svg"
  png="docs/images/readme-diagrams/id-jwt-boundary-${name}.png"
  xmllint --noout "$svg"
  cairosvg "$svg" -o "$png" -s 2
  identify "$png"
  python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-connector-audit.py" "$svg"
  python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-geometry-audit.py" --fail-diagonal "$svg"
  python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py" "$svg"
  python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py" "$svg"
done
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-sequence-style-audit.py" docs/images/readme-diagrams/id-jwt-boundary-sequence.svg
```

이 final render 뒤에 모든 PNG를 full detail과 original detail로 다시 열고 모든 arrow endpoint와 bend를 다시 검사한다.

기대값: SVG/PNG pair 세 개, 정확한 2x dimension, 의미 있는 count, 설명되지 않은 failure 0개, 올바른 rendered arrowhead direction, 안전한 marker/bend clearance, intrusion/crossing 없음, readable label.

- [ ] **Step 2: focused test 실행**

```bash
go test -count=1 ./examples/id-jwt-boundary/...
go test -race -count=1 ./examples/id-jwt-boundary/...
```

기대값: 모든 package가 pass하고 race detector가 race를 보고하지 않는다.

- [ ] **Step 3: repository CI 실행**

```bash
make ci
```

기대값: format, tidy, vet, lint, test, race gate가 pass한다.

- [ ] **Step 4: diff 및 checklist ledger review**

```bash
git status --short
git diff --check develop...HEAD
git diff --stat develop...HEAD
git log --oneline develop..HEAD
```

기대값: approved spec, plan, README pair, lesson, asset 여섯 개만 differ한다. Type E, Go public-evidence, diagram, workflow row는 fresh evidence를 가진다. `P0=0`, `P1=0`, `Blocked=0`.

- [ ] **Step 5: push 및 PR 생성**

`docs/issue-44-id-jwt-boundary-diagrams`를 push한다. base `develop`, milestone `0.6.0`, assignee `debop`, label `examples`, final level-two heading `## DoD Status`를 사용해 #44를 문서화하는 English PR을 생성한다.

- [ ] **Step 6: CI 대기 및 merge 전 중단**

CI가 성공한 뒤 live body, label, milestone, assignee, review/thread, check, head SHA를 확인한다. PR URL과 exact worktree path를 보고한다. 명시적인 사용자 승인 없이 merge하거나 remote branch를 삭제하지 않는다.
