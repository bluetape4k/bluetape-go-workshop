# Issue #44 ID/JWT Boundary Diagram 설계

## Status

2026-07-13 대화에서 승인되었다. 작성된 spec은 implementation plan과 diagram 작업이
시작되기 전에 review되고 승인되었다.

## Classification

- Work type: Type E maintenance.
- Scope: bilingual README pair, 세 쌍의 SVG/PNG asset, `examples/id-jwt-boundary`의
  기존 lesson.
- production behavior, Go source, test, dependency, module metadata, root README
  navigation, workflow는 변경하지 않는다.
- PR은 닫힌 issue #44를 문서화하고 milestone `0.6.0`, assignee `debop`, `examples`
  label을 이어받는다. feature issue를 다시 열거나 닫지 않는다.

## Goal

milestone `0.6.0`의 첫 번째 diagram-coverage gap을 닫는다. README는 reader가 curl
command와 source code를 다시 조립하지 않고도 다음 세 질문에 답할 수 있게 해야 한다.

1. caller는 local demo token을 어떻게 얻고, 이를 사용해 order를 어떻게 생성하는가?
2. HTTP validation, JWT trust, authorization policy, internal identifier generation은
   각각 어느 component가 소유하는가?
3. issuer, audience, expiration, role, scope는 어떤 순서로 확인되며, stable public
   failure는 flow의 어디에서 반환되는가?

## Source Model

diagram은 다음 현재 file에 근거한다.

- `examples/id-jwt-boundary/README.md`
- `examples/id-jwt-boundary/README.ko.md`
- `examples/id-jwt-boundary/main.go`
- `examples/id-jwt-boundary/internal/idjwtboundary/service.go`
- tests under `examples/id-jwt-boundary`
- `docs/superpowers/specs/2026-06-22-issue-44-id-jwt-boundary-design.md`
- `docs/lessons/2026-06-22-id-jwt-boundary.md`

source-backed invariant는 다음과 같다.

- `POST /tokens`는 bounded JSON을 받고 issuer `id-jwt-boundary`, audience
  `orders-api`, subject, role, space-separated scope claim을 가진 짧은 수명의 local
  demo JWT를 만든다.
- `POST /orders`는 bounded JSON을 받고 Bearer token을 요구하며 issuer, audience,
  expiration, role `customer`, scope `orders:create`를 검증한다.
- 검증되고 authorized된 request는 request validation 성공 이후에만 별도의 UUID v7 order
  identifier와 request identifier를 생성한다.
- UUID는 identifier이지 credential이 아니며, signed JWT claim은 encrypted가 아니다.
- missing, expired, unverifiable, forbidden, invalid-request, internal failure는 raw
  token, secret, parser diagnostic 없이 allowlisted public response로 mapping된다.
- `main` + `net/http`는 loopback server와 timeout을 소유한다. Gin Router는 routing,
  recovery, trusted-proxy configuration을 소유한다. route adapter는 8 KiB body limit과
  JSON/status mapping을 소유한다. `Service`는 local policy boundary를 소유한다.
  `jwt.Provider`는 signed claim composition/parsing을 소유한다. UUID v7 generator는
  internal identifier creation을 소유한다.
- 고정 HMAC key와 `/tokens` endpoint는 deterministic local-demo convenience일 뿐,
  production identity provider 또는 secret-management model이 아니다.

## Approaches Considered

### A. Three source-specific diagrams

scenario flow, static architecture view, chronological request sequence를 추가한다. 각
asset은 reader 질문 하나에 답하며 기존 workshop README family와 맞는다. 이것이 선택된
approach다.

### B. One combined trust-boundary overview

단일 image는 asset 수를 줄일 수 있지만 demo token-issue path, static ownership boundary,
success chronology, 네 가지 public auth failure를 섞게 된다. 결과물은 README scale에서 너무
빽빽해지거나 identity, authorization, ID generation 사이의 핵심 구분을 빠뜨릴 것이다.

### C. Architecture and sequence only

두 asset은 implementation ownership과 call order를 설명할 수 있지만 reader에게는 여전히
간결한 scenario-level adoption path가 부족하다. public README는 component detail을 보여주기
전에 demo workflow를 먼저 소개해야 한다.

## Asset Design

### Scenario

- Files:
  - `docs/images/readme-diagrams/id-jwt-boundary-scenario.svg`
  - `docs/images/readme-diagrams/id-jwt-boundary-scenario.png`
- Reader question: 실행 가능한 demo가 token과 order boundary를 end to end로 어떻게
  통과하는가?
- Shape: token request에서 검증된 order creation까지 이어지는 left-to-right numbered
  workflow와 하단 trust-notes band.
- Required steps: local token 요청, claim 서명, Bearer order request 전송, trust와
  authorization 검증, order 검증, UUID v7 order 및 request ID 생성, `201 Created` 반환.
- Required notes: token signature는 secrecy가 아니라 integrity를 제공한다. 생성된 ID는
  credential이 아니다. production secret은 source 밖에 있어야 한다.
- Visual baseline: best-practices `workflow-image-upload`와 현재 workshop scenario family.

### Architecture

- Files:
  - `docs/images/readme-diagrams/id-jwt-boundary-architecture.svg`
  - `docs/images/readme-diagrams/id-jwt-boundary-architecture.png`
- Reader question: 각 trust decision과 generated value는 누가 소유하는가?
- Shape: caller/demo gateway, Gin HTTP boundary, application service,
  `bluetape-go/jwt`, `bluetape-go/id`, public response의 responsibility region.
- Required relationships: `/tokens`에서 claim composition으로, `/orders`에서 Bearer
  extraction 및 bounded input으로, parse expectation에서 JWT provider로, role/scope
  policy는 service 내부로, UUID generation은 authorized success path에서만 이어져야 한다.
- demo issuer와 protected order path는 시각적으로 분명히 구분되어야 한다. diagram은 encoding이
  claim을 encrypt한다거나 UUID v7이 authorization을 부여한다고 암시하면 안 된다.
- Visual baselines: grouped responsibility는 best-practices
  `utils-idgenerators-diagram-03`, explicit application/integration/API/backend
  ownership region은 승인된 framework-boundary architecture
  `leader-ktor-architecture-01`.

### Sequence

- Files:
  - `docs/images/readme-diagrams/id-jwt-boundary-sequence.svg`
  - `docs/images/readme-diagrams/id-jwt-boundary-sequence.png`
- Reader question: 정확한 success order는 무엇이며 각 auth failure는 어디에서 반환되는가?
- Participants: caller, Gin router, boundary service, JWT provider, policy/ID
  generation, HTTP response.
- Required success messages: bounded JSON parse, Bearer extraction,
  issuer/audience/expiration expectation을 포함한 token parse, role/scope check,
  order validation, 두 번의 UUID v7 generation, `201` response.
- Required failure representation: missing token, expired token, 기타 invalid token,
  verified-but-forbidden role/scope는 ID generation 전에 stable `401`/`403` response로
  mapping한다. 전체 success sequence를 중복하지 말고 failure branch를 compact하게 유지한다.
- Visual baselines: best-practices `sequence-workflow-sample`과 가장 가까운 repo-local
  Gin sequence인 `docs/images/readme-diagrams/gin-audit-query-api-sequence.png`. Gin
  sequence는 local participant, activation, message, branch-frame, palette family에
  대한 authoritative 기준이다.

## README and Lesson Integration

두 locale file은 같은 PNG를 같은 순서로 embed한다.

1. 기존 `Scenario` section의 scenario
2. `What It Demonstrates` 앞의 architecture
3. `Boundary Notes` 앞의 request sequence

English prose는 public source text로 유지한다. Korean prose는 endpoint name, claim,
role, scope, error code, trust semantic을 보존하면서 자연스럽게 localize한다. 두 locale이
같은 asset을 공유하도록 diagram label은 English로 유지한다.

기존 lesson에는 diagram evidence와 durable review rule을 추가한다. rendered PNG inspection은
conversion 이후 arrowhead direction, marker size, endpoint clearance, bend coordinate를
검증해야 한다. 같은 example에 대해 두 번째 lesson을 만들지 않는다.

## Visual Contract

- 현재 `bluetape-diagram` checklist와 wiki `docs/diagrams/best-practices` golden
  catalog를 따른다.
- workshop light palette, visible outer frame, 명확한 title/subtitle, 정렬된
  responsibility band, 읽기 쉬운 English label을 사용한다.
- 최종 README asset에는 Mermaid, ASCII, Graphviz output, emoji, invented logo, flat
  grid를 사용하지 않는다.
- 짧은 orthogonal connector, 색상별 명시적 fixed-size marker, route가 실제로 방향을 바꾸는
  곳의 rounded bend만 사용한다.
- primary/scenario marker는 `markerWidth="14"`, `markerHeight="14"`,
  `viewBox="0 0 14 14"`, `refX="12"`, `refY="7"`, and the filled triangle
  `M2 2 L12 7 L2 12 Z`.
- sequence marker는 `markerWidth="16"`, `markerHeight="16"`,
  `viewBox="0 0 16 16"`, `refX="13"`, `refY="8"`, and the filled triangle
  `M3 3 L13 8 L3 13 Z`.
- 모든 directional marker는 `markerUnits="userSpaceOnUse"`, `orient="auto"`, 명시적
  semantic-color fill, connector 색상별 별도 marker를 사용한다.
- primary 및 sequence connector는 final bend 이후 최소 24 px의 straight terminal distance를
  확보한다. bend가 target에 너무 가까워 PNG arrowhead가 뒤집히거나, 떨어져 보이거나, 이전
  segment 방향을 향하면 안 된다.
- connector는 card 내부가 아니라 card boundary에 수직으로 붙으며, endpoint corner clearance는
  최소 `max(8 px, rx / 2)`여야 한다.
- SVG geometry와 변환된 PNG를 모두 inspect한다. XML/render command 통과는 visual evidence가
  아니다.
- 한 번에 하나의 asset만 작업한다. SVG edit, XML validate, 2x PNG render, automated audit,
  full-size PNG inspection, native-pixel crop inspection을 마친 뒤 다음으로 진행한다.

## Validation

각 asset에 대해 다음을 수행한다.

- scenario, architecture, sequence SVG file에 `xmllint --noout` 실행
- 각 named SVG를 CairoSVG scale 2로 matching PNG에 render
- 의미 있는 connector count와 설명되지 않은 failure 0건을 확인하는 connector, geometry,
  endpoint, mixed-corner audit
- 해당되는 경우 architecture 또는 sequence-specific audit
- 명시적 SVG-to-PNG arrowhead-direction review
- marker-size 대비 final-segment 및 bend-clearance에 대한 명시적 review
- 최종 coordinate 변경 이후 label fit, endpoint contact, line/card intrusion, crossing,
  font, margin, whitespace에 대한 full-size 및 native-pixel PNG inspection

README와 branch에 대해서는 다음을 수행한다.

- English/Korean asset order, prose parity, link resolution 검증
- `git diff --check`;
- `go test -count=1 ./examples/id-jwt-boundary/...`;
- `go test -race -count=1 ./examples/id-jwt-boundary/...`;
- PR completion 전에 `make ci`.

PR body는 `## DoD Status`로 끝난다. merge approval을 요청하기 전에 CI, review thread,
milestone, assignee, label, live body shape를 다시 확인한다. 명시적 user approval 없이 merge
또는 remote-branch deletion을 수행하지 않는다.

## Non-Goals

- production identity provider, asymmetric key distribution, key rotation, refresh token,
  session, database, secret-manager implementation은 하지 않는다.
- Go behavior, test, endpoint, request/response shape, status code, default secret,
  library dependency는 변경하지 않는다.
- root README redesign 또는 이후 `0.6.0` example용 diagram은 만들지 않는다.
- demo `/tokens` endpoint가 production에 적합하다고 주장하지 않는다.
