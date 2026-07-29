# Issue #58 Gin Audit Query API 다이어그램 리뷰

## 범위와 권한

- 분류: Type E documentation/visual maintenance. runtime behavior나 dependency 변경은 없다.
- 기준: `2c56fcf`의 `origin/develop`.
- 전달 경계: PR을 만들고 green CI를 기다린다. merge와 worktree cleanup은 별도 명시 승인이 필요하다.
- Public surface: English/Korean 예제 README pair.
- 권위 있는 image policy: 편집 가능한 SVG source와 렌더링된 PNG. README는 PNG를 embed한다.

## 승인된 참조

| 다이어그램 | 승인된 best-practice reference | 보존된 visual contract |
|---|---|---|
| Reader scenario | `workflow-image-upload.svg/png` | 번호가 있는 main row, 아래쪽 supporting concern, 짧은 reader-first subtitle. |
| Component architecture | `leader-ktor-architecture-01.svg/png` | static ownership zone, 분리된 integration/library/backend concern, compact card. |
| Search sequence | `sequence-workflow-sample.svg/png` | 직사각형 participant header, dashed lifeline, activation bar, numbered pill, dashed return, 명시적 `alt/else`. |

모든 참조는 `bluetape4k-wiki/docs/diagrams/best-practices/assets`에서 왔다.
두 localized README 파일이 같은 asset을 공유할 수 있도록 diagram label은
English로 유지한다.

## Source 사실 기록

| 다이어그램 주장 | Source evidence |
|---|---|
| `order-1001`은 `order.created`부터 `order.shipped`까지 revision 1-4를 가진다 | `examples/gin-audit-query-api/internal/auditquery/fixture.go` |
| Search는 `POST /audit/history/search`이고 exact revision은 GET route다 | `examples/gin-audit-query-api/internal/auditquery/server.go` |
| JSON body limit은 32 KiB이고 request timeout은 2초다 | `server.go`의 `DefaultHTTPConfig` |
| Query service는 `limit + 1`을 읽고 요청된 entry를 반환하며 inclusive next revision을 낸다 | `service.go`의 `Service.Search` |
| Query service는 `audit.HistoryReader`에 의존하고 runnable app은 `audit.NewMemoryRepository`를 연결한다 | `service.go` 및 `main.go` |
| Remote binding에는 `ALLOW_UNAUTHENTICATED_REMOTE=1`이 필요하다 | `main.go`의 `resolveHTTPAddr` |

## Asset 및 audit 기록

| Asset | 독자 질문 | Parse/render | Connector audit | Geometry/endpoint/corner | Type audit | Full-size PNG review |
|---|---|---|---|---|---|---|
| `gin-audit-query-api-scenario.svg/png` | 독자는 end-to-end로 무엇을 하는가? | PASS, 2880x1440 | PASS, 7 connectors, 0 intrusion/crossing | PASS | numbered step 5개와 lower support band 검증 | unsupported glyph 교체 뒤 PASS |
| `gin-audit-query-api-architecture.svg/png` | 각 책임은 어느 layer가 소유하는가? | PASS, 3120x1840 | PASS, 4 connectors, 0 intrusion/crossing | PASS | static ownership 및 framework/library/fixture boundary 검증 | `MemoryRepository` title margin 확장 뒤 PASS |
| `gin-audit-query-api-sequence.svg/png` | 하나의 POST search는 어떤 순서로 실행되는가? | PASS, 3360x2520 | PASS, 7 connectors, 0 intrusion/crossing | PASS | sequence-style audit PASS; message 1-7과 `alt/else` 검증 | activation start를 endpoint collision에서 옮긴 뒤 PASS |

인프라 아이콘 점검은 해당하지 않는다. 표시된 모든 요소는 code, interface,
또는 in-memory example fixture다. 외부 database, broker, cloud, durable
infrastructure는 주장하지 않는다.

## 시각 점검 기록

각 PNG는 마지막 좌표 또는 텍스트 변경 후 원본 해상도로 개별 확인했다. 점검은
title/subtitle 가독성, 잘리거나 누락된 glyph, card text margin, label-to-line
clearance, arrowhead, endpoint placement, line crossing, branch frame, canvas
edge, whitespace, visual family consistency를 다뤘다. contact-sheet inspection을
대체 검증으로 사용하지 않았다. 보조 three-image contact sheet로 asset family의
font, palette, frame, corner, whitespace 일관성을 확인했다. 이후 사용자가 로컬
파일을 열고 최종 시각 결과를 승인했다.

## 로컬 검증

```text
xmllint --noout <all three SVGs>                              PASS
diagram-connector-audit.py <all three SVGs>                  PASS
diagram-geometry-audit.py --fail-diagonal <all three SVGs>   PASS
diagram-endpoint-audit.py <all three SVGs>                   PASS
diagram-mixed-corner-audit.py <all three SVGs>               PASS
diagram-sequence-style-audit.py <sequence SVG>               PASS
README image existence and locale-order checks               PASS
git diff --check                                             PASS
make lint                                                    PASS, 0 issues
make ci                                                      PASS
```

첫 `make ci` lint 시도는 삭제된 `feat-issue-58-gin-audit-query-api` worktree를
여전히 참조하는 stale shared golangci-lint cache를 드러냈다. 실패는 shared
cache에서 재현되었고 fresh isolated cache에서는 통과했다. `golangci-lint cache
clean` 뒤 default-environment `make lint`와 fresh full `make ci`가 source 변경
없이 모두 통과했다.

## 차단 체크리스트

| 체크리스트 | 상태 | 증거 또는 보류 |
|---|---|---|
| `CL-01..CL-08` | IN PROGRESS | mutation 전 생성됨. Type E 항목은 분류 및 순서화됨. 최종 count는 PR/CI 증거를 기다린다. |
| `WF-01..WF-06` | IN PROGRESS | 첫 plan 승인됨. execution contract 로드됨. 약한 audit 및 visual finding은 즉시 수정됨. |
| `CG-01..CG-08` | PASS | authority 재확인, 현재 repo/code 점검, 사용자 작업 worktree 격리, public bilingual docs 및 ecosystem image pattern 보존. |
| `CG-09..CG-10` | PENDING | push 뒤 live PR metadata, body, check, comment, review thread를 검증한다. |
| `CG-11` | HOLD | 별도 명시 승인 없이 merge, remote branch deletion, worktree cleanup 금지. |
| `CG-12` | N/A | post-merge synchronization은 현재 승인 단계 밖이다. |
| `CG-13..CG-16` | N/A | managed/generated policy source 또는 durable Codex resource 변경 없음. authoritative local image tooling을 직접 사용함. |
| `CG-17` | PENDING | 최종 line-by-line count는 PR/CI evidence를 기다린다. |
| `E-01..E-05` | PASS | diagram 및 writer skill routed. behavior 보존. locale parity와 maintenance verification 통과. |
| `E-06` | PENDING | durable PR delivery가 남아 있다. |
| `DIA-01..DIA-06` | PASS | scope, rule, one-at-a-time SVG edit, PNG render, automated audit, individual visual review가 위에 기록됨. |
| `DIA-07..DIA-08` | IN PROGRESS | README exposure는 존재한다. final diff hygiene과 PR evidence가 남아 있다. |
| `DIA-COM-01..DIA-COM-02` | PASS | source 및 related set 확인. 승인된 font, palette, readable text, shared English label 사용. |
| `DIA-COM-03` | N/A | 실제 infrastructure는 표시하지 않는다. |
| `DIA-COM-04..DIA-COM-08` | PASS | PNG arrowhead, route, corner, canvas, render command, audit를 확인했다. |
| `DIA-COM-09` | PENDING | review exposure는 PR을 기다린다. |
| `DIA-ARC-01..DIA-ARC-04` | PASS | scenario와 architecture는 서로 다른 reader question에 답한다. architecture는 static이고 dependency가 검증되어 있다. |
| `DIA-SEQ-01..DIA-SEQ-06` | PASS | 승인된 reference를 열었고 sequence signal/palette/marker/message/branch를 검증했으며 sequence audit가 통과했다. |
