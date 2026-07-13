# Issue #44 ID/JWT Boundary Diagram Design

## Status

Approved in conversation on 2026-07-13. This written spec must be reviewed
before the implementation plan and diagram work begin.

## Classification

- Work type: Type E maintenance.
- Scope: the bilingual README pair, three paired SVG/PNG assets, and the
  existing lesson for `examples/id-jwt-boundary`.
- Production behavior, Go source, tests, dependencies, module metadata, root
  README navigation, and workflows remain unchanged.
- The PR will document closed issue #44 and inherit milestone `0.6.0`, assignee
  `debop`, and the `examples` label. It will not reopen or close the feature
  issue.

## Goal

Close the first diagram-coverage gap in milestone `0.6.0`. The README should
let a reader answer three questions without reconstructing the boundary from
curl commands and source code:

1. How does a caller obtain a local demo token and use it to create an order?
2. Which component owns HTTP validation, JWT trust, authorization policy, and
   internal identifier generation?
3. In what order are issuer, audience, expiration, role, and scope checked, and
   where do stable public failures leave the flow?

## Source Model

The diagrams are grounded in these current files:

- `examples/id-jwt-boundary/README.md`
- `examples/id-jwt-boundary/README.ko.md`
- `examples/id-jwt-boundary/main.go`
- `examples/id-jwt-boundary/internal/idjwtboundary/service.go`
- tests under `examples/id-jwt-boundary`
- `docs/superpowers/specs/2026-06-22-issue-44-id-jwt-boundary-design.md`
- `docs/lessons/2026-06-22-id-jwt-boundary.md`

Source-backed invariants:

- `POST /tokens` accepts bounded JSON and creates a short-lived local demo JWT
  with issuer `id-jwt-boundary`, audience `orders-api`, subject, role, and
  space-separated scope claims;
- `POST /orders` accepts bounded JSON, requires a Bearer token, and validates
  issuer, audience, expiration, role `customer`, and scope `orders:create`;
- a verified and authorized request generates separate UUID v7 order and
  request identifiers only after request validation succeeds;
- UUIDs are identifiers, not credentials, and signed JWT claims are not
  encrypted;
- missing, expired, unverifiable, forbidden, invalid-request, and internal
  failures map to allowlisted public responses without raw token, secret, or
  parser diagnostics;
- Gin owns routing, recovery, body limits, and HTTP serialization; `Service`
  owns the local policy boundary; `jwt.Provider` owns signed claim
  composition/parsing; the UUID v7 generator owns internal identifier creation;
- the fixed HMAC key and `/tokens` endpoint are deterministic local-demo
  conveniences, not a production identity provider or secret-management model.

## Approaches Considered

### A. Three source-specific diagrams

Add a scenario flow, a static architecture view, and a chronological request
sequence. Each asset answers one reader question and matches the established
workshop README family. This is the selected approach.

### B. One combined trust-boundary overview

A single image would reduce asset count, but it would mix the demo token-issue
path, static ownership boundaries, success chronology, and four public auth
failures. The result would either be too dense at README scale or omit the
critical distinction between identity, authorization, and ID generation.

### C. Architecture and sequence only

Two assets would explain implementation ownership and call order, but readers
would still lack a compact scenario-level adoption path. The public README
should introduce the demo workflow before exposing component detail.

## Asset Design

### Scenario

- Files:
  - `docs/images/readme-diagrams/id-jwt-boundary-scenario.svg`
  - `docs/images/readme-diagrams/id-jwt-boundary-scenario.png`
- Reader question: how does the runnable demo cross the token and order
  boundary end to end?
- Shape: a left-to-right numbered workflow from token request through verified
  order creation, with a lower trust-notes band.
- Required steps: request local token, sign claims, send Bearer order request,
  verify trust and authorization, validate order, generate UUID v7 order and
  request IDs, return `201 Created`.
- Required notes: token signatures provide integrity rather than secrecy;
  generated IDs are not credentials; production secrets belong outside source.
- Visual baseline: best-practices `workflow-image-upload` and the current
  workshop scenario family.

### Architecture

- Files:
  - `docs/images/readme-diagrams/id-jwt-boundary-architecture.svg`
  - `docs/images/readme-diagrams/id-jwt-boundary-architecture.png`
- Reader question: who owns each trust decision and generated value?
- Shape: responsibility regions for caller/demo gateway, Gin HTTP boundary,
  application service, `bluetape-go/jwt`, `bluetape-go/id`, and public response.
- Required relationships: `/tokens` to claim composition, `/orders` to Bearer
  extraction and bounded input, parse expectations to JWT provider,
  role/scope policy inside the service, and UUID generation only on the
  authorized success path.
- The demo issuer and protected order path must remain visibly distinct. The
  diagram must not imply that encoding encrypts claims or that UUID v7 grants
  authorization.
- Visual baselines: best-practices `utils-idgenerators-diagram-03` for grouped
  responsibility and the nearest approved framework-boundary architecture
  sample.

### Sequence

- Files:
  - `docs/images/readme-diagrams/id-jwt-boundary-sequence.svg`
  - `docs/images/readme-diagrams/id-jwt-boundary-sequence.png`
- Reader question: what is the exact success order and where does each auth
  failure return?
- Participants: caller, Gin router, boundary service, JWT provider, policy/ID
  generation, and HTTP response.
- Required success messages: bounded JSON parse, Bearer extraction, token
  parse with issuer/audience/expiration expectations, role/scope check, order
  validation, two UUID v7 generations, and `201` response.
- Required failure representation: missing token, expired token, other invalid
  token, and verified-but-forbidden role/scope map to stable `401`/`403`
  responses before ID generation. Keep failure branches compact instead of
  duplicating the full success sequence.
- Visual baseline: best-practices `sequence-workflow-sample`.

## README and Lesson Integration

Both locale files embed the same PNGs in the same order:

1. scenario in the existing `Scenario` section;
2. architecture before `What It Demonstrates`;
3. request sequence before `Boundary Notes`.

English prose remains the public source text. Korean prose is localized
naturally while preserving endpoint names, claims, roles, scopes, error codes,
and trust semantics. Diagram labels remain English so both locales share the
same assets.

Update the existing lesson with diagram evidence and the durable review rule:
rendered PNG inspection must verify arrowhead direction, marker size, endpoint
clearance, and bend coordinates after conversion. Do not create a second lesson
for the same example.

## Visual Contract

- Follow the current `bluetape-diagram` checklist and the wiki
  `docs/diagrams/best-practices` golden catalog.
- Use the workshop light palette, visible outer frame, clear title/subtitle,
  aligned responsibility bands, and readable English labels.
- Do not use Mermaid, ASCII, Graphviz output, emoji, invented logos, or a flat
  grid for the final README assets.
- Use short orthogonal connectors, explicit per-color fixed-size markers, and
  rounded bends only where the route actually changes direction.
- Reserve terminal distance for the rendered arrowhead. A bend must not sit so
  close to a target that the PNG arrowhead appears reversed, detached, or aimed
  along the previous segment.
- Inspect both SVG geometry and the converted PNG. Passing XML/render commands
  is not visual evidence.
- Work one asset at a time: edit SVG, validate XML, render 2x PNG, run automated
  audits, inspect full-size PNG, inspect native-pixel crops, then proceed.

## Validation

For every asset:

- run `xmllint --noout` against the scenario, architecture, and sequence SVG
  files;
- render each named SVG to its matching PNG with CairoSVG at scale 2;
- connector, geometry, endpoint, and mixed-corner audits with meaningful
  connector counts and zero unexplained failures;
- architecture or sequence-specific audit where applicable;
- explicit SVG-to-PNG arrowhead-direction review;
- explicit marker-size versus final-segment and bend-clearance review;
- full-size and native-pixel PNG inspection after the final coordinate change
  for label fit, endpoint contact, line/card intrusion, crossings, fonts,
  margins, and whitespace.

For the README and branch:

- verify English/Korean asset order, prose parity, and link resolution;
- `git diff --check`;
- `go test -count=1 ./examples/id-jwt-boundary/...`;
- `go test -race -count=1 ./examples/id-jwt-boundary/...`;
- `make ci` before PR completion.

The PR body ends with `## DoD Status`. CI, review threads, milestone, assignee,
labels, and live body shape are rechecked before requesting merge approval. No
merge or remote-branch deletion occurs without explicit user approval.

## Non-Goals

- No production identity provider, asymmetric key distribution, key rotation,
  refresh token, session, database, or secret-manager implementation.
- No changes to Go behavior, tests, endpoints, request/response shapes, status
  codes, default secret, or library dependencies.
- No root README redesign or diagrams for later `0.6.0` examples.
- No claim that the demo `/tokens` endpoint is suitable for production.
