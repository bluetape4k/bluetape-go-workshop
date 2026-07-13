# Issue #44 ID/JWT Boundary Diagrams Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three source-backed, bilingual README diagrams that explain the ID/JWT demo scenario, runtime trust ownership, and protected order request sequence.

**Architecture:** This is a documentation-only change. Each diagram is a hand-authored SVG in the canonical README asset directory, rendered to a paired 2x PNG with CairoSVG, audited independently, visually inspected at full and original detail, and embedded in both README locales in identical order.

**Tech Stack:** SVG, CairoSVG, XMLLint, bluetape-diagram audit scripts, ImageMagick metadata inspection, Markdown, Go repository verification.

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

The root README files, Go source, tests, module metadata, and workflows are out of scope.

## Task 1: Lock Source and Visual Evidence

- [ ] **Step 1: Confirm source invariants**

Run:

```bash
sed -n '1,220p' examples/id-jwt-boundary/README.md
sed -n '1,380p' examples/id-jwt-boundary/internal/idjwtboundary/service.go
sed -n '1,220p' examples/id-jwt-boundary/internal/idjwtboundary/service_test.go
```

Expected: bounded `/tokens` and `/orders` JSON, fixed-HMAC demo composition, expected issuer/audience/expiration parsing, `customer` role, `orders:create` scope, validation before two UUID v7 generations, and stable public error mapping are present.

- [ ] **Step 2: Inspect approved visual baselines**

Open these PNGs at original detail:

```text
/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/workflow-image-upload.png
/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/utils-idgenerators-diagram-03.png
/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/sequence-workflow-sample.png
```

Expected: scenario stages read in one direction, architecture uses responsibility regions rather than a box grid, and sequence messages have rectangular participants, dashed lifelines, numbered labels, visible arrowheads, and transparent branch frames.

- [ ] **Step 3: Lock marker geometry**

Use these exact invariants:

```text
primary/scenario marker: 14x14 user-space marker, 10x10 filled triangle
sequence marker: 13x13 user-space marker, 10x10 filled triangle
marker orientation: orient="auto"
primary terminal segment after final bend: >= 20 px
sequence terminal segment after final bend: >= 16 px
connector endpoint: card boundary, never inside the card
```

Expected: the final segment determines rendered arrow direction, and no bend occupies the marker footprint.

## Task 2: Create and Prove the Scenario Diagram

- [ ] **Step 1: Create the scenario SVG**

Create `docs/images/readme-diagrams/id-jwt-boundary-scenario.svg` on a 1200x650 light canvas with a rounded outer decorator, title, subtitle, six numbered left-to-right cards, and one lower trust-notes band:

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

The trust-notes band contains:

```text
Signed claims are not encrypted
UUIDs are identifiers, not credentials
The fixed HMAC secret is local demo material only
```

Use blue for request progress, violet for JWT trust, and olive for authorized ID creation. Every progression line is orthogonal, terminates on a card edge, and leaves at least 20 px after its final bend.

- [ ] **Step 2: Validate and render the scenario**

Run:

```bash
xmllint --noout docs/images/readme-diagrams/id-jwt-boundary-scenario.svg
cairosvg docs/images/readme-diagrams/id-jwt-boundary-scenario.svg -o docs/images/readme-diagrams/id-jwt-boundary-scenario.png -s 2
identify docs/images/readme-diagrams/id-jwt-boundary-scenario.png
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-connector-audit.py" docs/images/readme-diagrams/id-jwt-boundary-scenario.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-geometry-audit.py" --fail-diagonal docs/images/readme-diagrams/id-jwt-boundary-scenario.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py" docs/images/readme-diagrams/id-jwt-boundary-scenario.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py" docs/images/readme-diagrams/id-jwt-boundary-scenario.svg
```

Expected: XML/render success, PNG 2400x1300, five primary progression connectors, and zero diagonal, endpoint, or mixed-corner failures.

- [ ] **Step 3: Inspect the rendered scenario**

Open the 2400x1300 PNG at full and original detail. Inspect all five endpoints at native pixels. Verify every arrow points toward the next card, no head is clipped or detached, no line crosses a card, labels fit, the trust band stays secondary, and margins are balanced.

- [ ] **Step 4: Commit the scenario pair**

```bash
git add docs/images/readme-diagrams/id-jwt-boundary-scenario.svg docs/images/readme-diagrams/id-jwt-boundary-scenario.png
git commit -m "docs: add ID JWT boundary scenario diagram"
```

## Task 3: Create and Prove the Architecture Diagram

- [ ] **Step 1: Create the architecture SVG**

Create `docs/images/readme-diagrams/id-jwt-boundary-architecture.svg` on a 1400x900 light canvas. Use responsibility regions and aligned cards, not a flat grid:

```text
Caller and demo boundary
  Demo Client -> POST /tokens
  Order Client -> Bearer JWT + order JSON

Gin HTTP boundary
  loopback HTTP server + timeouts
  Recovery + no trusted proxies
  8 KiB body limit + JSON/status mapping

Application trust boundary
  IssueToken: request validation + claim assembly
  CreateOrder: Bearer extraction + trust checks
  Local policy: customer + orders:create
  Order validation + allowlisted public errors

bluetape-go providers
  jwt.FixedHMACProvider: compose/parse signed claims
  id.UUIDV7Generator: order_id and request_id after authorization

Public outcomes
  200 demo token
  201 accepted order
  400/401/403/500 stable error response
```

Use blue for HTTP/application ownership, violet for signed-claim composition and verification, olive for the authorized UUID path, and muted red for failure mapping. Include an in-image legend. Keep `/tokens` and `/orders` separate until their shared service boundary. Never connect UUID generation to a failure path.

- [ ] **Step 2: Validate and render the architecture**

Run:

```bash
xmllint --noout docs/images/readme-diagrams/id-jwt-boundary-architecture.svg
cairosvg docs/images/readme-diagrams/id-jwt-boundary-architecture.svg -o docs/images/readme-diagrams/id-jwt-boundary-architecture.png -s 2
identify docs/images/readme-diagrams/id-jwt-boundary-architecture.png
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-connector-audit.py" docs/images/readme-diagrams/id-jwt-boundary-architecture.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-geometry-audit.py" --fail-diagonal docs/images/readme-diagrams/id-jwt-boundary-architecture.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-endpoint-audit.py" docs/images/readme-diagrams/id-jwt-boundary-architecture.svg
python3 "${CODEX_HOME:-$HOME/.codex}/skills/bluetape-diagram/scripts/diagram-mixed-corner-audit.py" docs/images/readme-diagrams/id-jwt-boundary-architecture.svg
```

Expected: XML/render success, PNG 2800x1800, at least twelve cards and ten connectors, and zero automated audit failures.

- [ ] **Step 3: Inspect architecture connector geometry**

Open the final PNG at full and original detail. For every connector compare the SVG final segment with the rendered head and verify:

```text
arrowhead direction matches the final path segment
final segment is >= 20 px after the last bend
marker tip contacts the intended card boundary
bend/corner does not overlap the marker footprint
no connector crosses a card, label, lane title, or another line
no connector rides a card edge or appears tangent
```

On any failure, move ports or bend coordinates, rerender, rerun every architecture audit, and repeat the full-size inspection.

- [ ] **Step 4: Commit the architecture pair**

```bash
git add docs/images/readme-diagrams/id-jwt-boundary-architecture.svg docs/images/readme-diagrams/id-jwt-boundary-architecture.png
git commit -m "docs: add ID JWT boundary architecture diagram"
```

## Task 4: Create and Prove the Request Sequence

- [ ] **Step 1: Create the sequence SVG**

Create `docs/images/readme-diagrams/id-jwt-boundary-sequence.svg` on a 1600x1300 light canvas with participants `Caller`, `Gin Router`, `Boundary Service`, `JWT Provider`, and `UUID v7 Generator`. Use rectangular participant headers, five dashed lifelines without arrowheads, activation bars, transparent branch frames, numbered message pills, and continuous message lines.

Show this success order:

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

Add compact failure branches at their actual decision levels:

```text
missing token -> 401 missing_token before JWT parse
expired token -> 401 expired_token after JWT parse
other parse/signature/key failure -> 401 invalid_token
verified wrong role/scope -> 403 forbidden before UUID generation
invalid order -> 400 invalid_request before UUID generation
ID generation failure -> 500 internal_error without internals
```

Use solid calls, dashed returns, red only for failures, violet for JWT messages, and olive for UUID creation. Each 13x13 marker uses `markerUnits="userSpaceOnUse"`, `orient="auto"`, matching fill color, and at least 16 px of straight terminal route after the last bend.

- [ ] **Step 2: Validate and render the sequence**

Run:

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

Expected: PNG 3200x2600, five participant headers/lifelines, twelve numbered success messages, transparent nonempty failure branches, zero common failures, and a passing sequence-style audit.

- [ ] **Step 3: Inspect sequence direction**

Open the final PNG at full and original detail. Verify solid calls point right, dashed returns point left, labels sit above their uninterrupted lines, activation bars do not hide marker heads, branch frames preserve chronology, and no failure branch reaches UUID generation.

- [ ] **Step 4: Commit the sequence pair**

```bash
git add docs/images/readme-diagrams/id-jwt-boundary-sequence.svg docs/images/readme-diagrams/id-jwt-boundary-sequence.png
git commit -m "docs: add ID JWT boundary sequence diagram"
```

## Task 5: Integrate README Locales and Lesson

- [ ] **Step 1: Update the English README**

Under `## Scenario`, add the scenario PNG and explain that it separates the local demo issuer from the protected order boundary. Before `## What It Demonstrates`, add `## Architecture` with the architecture PNG and ownership explanation. Before `## Boundary Notes`, add `## Protected Order Sequence` with the sequence PNG and state that auth failures exit before UUID generation.

Use these links:

```markdown
![ID and JWT boundary scenario](../../docs/images/readme-diagrams/id-jwt-boundary-scenario.png)
![ID and JWT boundary architecture](../../docs/images/readme-diagrams/id-jwt-boundary-architecture.png)
![ID and JWT protected order sequence](../../docs/images/readme-diagrams/id-jwt-boundary-sequence.png)
```

- [ ] **Step 2: Update the Korean README**

Add the same three paths in the same order under the corresponding headings. Localize naturally while preserving `/tokens`, `/orders`, `issuer`, `audience`, `expiration`, `customer`, `orders:create`, UUID v7, and public error codes.

- [ ] **Step 3: Update the lesson**

Append `## Diagram Evidence` to `docs/lessons/2026-06-22-id-jwt-boundary.md` with these points:

```text
The three assets are source-backed and shared by both README locales.
SVG-to-PNG review checks arrowhead direction, marker color parity, endpoint contact, and native-pixel readability.
Connector bends reserve straight terminal distance for the marker footprint; automated geometry success alone is insufficient.
JWT failure paths visibly end before UUID generation, so identifiers never appear to authorize a request.
```

- [ ] **Step 4: Verify locale order and links**

Run:

```bash
diff <(rg -o 'readme-diagrams/[^) ]+\.png' examples/id-jwt-boundary/README.md) <(rg -o 'readme-diagrams/[^) ]+\.png' examples/id-jwt-boundary/README.ko.md)
for asset in $(rg -o 'readme-diagrams/[^) ]+\.png' examples/id-jwt-boundary/README.md | sed 's#readme-diagrams/##'); do test -f "docs/images/readme-diagrams/$asset"; test -f "docs/images/readme-diagrams/${asset%.png}.svg"; done
git diff --check
```

Expected: locale diff empty, all three PNG/SVG pairs resolve, and diff check exits zero.

- [ ] **Step 5: Commit README and lesson integration**

```bash
git add examples/id-jwt-boundary/README.md examples/id-jwt-boundary/README.ko.md docs/lessons/2026-06-22-id-jwt-boundary.md
git commit -m "docs: explain ID JWT boundary diagrams"
```

## Task 6: Final Verification and PR Delivery

- [ ] **Step 1: Rerun every final diagram gate**

Run:

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

Reopen every PNG at full and original detail after this final render and inspect
every arrow endpoint and bend again.

Expected: three SVG/PNG pairs, exact 2x dimensions, meaningful counts, zero unexplained failures, correct rendered arrowhead direction, safe marker/bend clearance, no intrusions/crossings, and readable labels.

- [ ] **Step 2: Run focused tests**

```bash
go test -count=1 ./examples/id-jwt-boundary/...
go test -race -count=1 ./examples/id-jwt-boundary/...
```

Expected: all packages pass and the race detector reports no race.

- [ ] **Step 3: Run repository CI**

```bash
make ci
```

Expected: format, tidy, vet, lint, test, and race gates pass.

- [ ] **Step 4: Review diff and checklist ledger**

```bash
git status --short
git diff --check develop...HEAD
git diff --stat develop...HEAD
git log --oneline develop..HEAD
```

Expected: only the approved spec, plan, README pair, lesson, and six assets differ. Type E, Go public-evidence, diagram, and workflow rows have fresh evidence; `P0=0`, `P1=0`, and `Blocked=0`.

- [ ] **Step 5: Push and create the PR**

Push `docs/issue-44-id-jwt-boundary-diagrams`. Create an English PR documenting #44 with base `develop`, milestone `0.6.0`, assignee `debop`, label `examples`, and `## DoD Status` as the final level-two heading.

- [ ] **Step 6: Wait for CI and stop before merge**

Verify live body, labels, milestone, assignee, reviews/threads, checks, and head SHA after CI succeeds. Report the PR URL and exact worktree path. Do not merge or delete the remote branch without explicit user approval.
