# Issue #50 graph abuse cluster verification

## 결론

- 기준: `origin/develop@ed1fb311c256bf1c95c9545f5c418a5460c9b278`
- 검증 대상: `feat/issue-50-graph-abuse-cluster`의 code/diagram HEAD `a53ac37`
- 구현 기준: `bluetape-go v0.18.0`
- 최종 review: P0=0, P1=0, P2=0
- 코드/문서/diagram focused gate: fresh exit 0
- 전체 release gate: clean GitHub runner의 `make ci` 성공 전에는 merge 불가

Neo4j는 한정된 fixture의 저장과 조회만 담당한다. released `graph/neo4j`
adapter는 caller-owned driver를 통해 graph 값을 운반하고, connected component,
evidence, 5/3/1 score, 정렬은 예제의 Go policy로 남겼다. reusable graph
repository나 algorithm abstraction은 추가하지 않았다.

## Acceptance evidence

| Contract | Evidence |
|---|---|
| persistence 전 graph value 검증 | `fixture_test.go`의 schema, duplicate, endpoint, exact/max+1 경계와 defensive-copy 표 |
| scoped atomic reset/seed | `store.go`의 단일 `ExecuteWrite`와 fixture parameter; integration idempotence/rollback/unrelated namespace 증거 |
| 두 번의 bounded read | vertex `LIMIT 257`, edge `LIMIT 1025`; 256/1024 초과 시 `ErrGraphTooLarge` |
| deterministic analysis | iterative component walk, input shuffle, payment-token weight, score/user-count/ID tie-break tests |
| safe CLI lifecycle | strict loopback `bolt://`, 15초 operation, fresh 3초 cleanup, encode -> close -> checked stdout |
| real backend | Neo4j Testcontainers round trip, cancellation, repeat replace, namespace cleanup |
| runnable lesson | bilingual README parity, exact JSON, Terminal 1/2 commands, fixed namespace and production caveats |
| visual explanation | architecture/sequence canonical SVG+PNG, deterministic render, structural audits and original-size eye checks |

## Fresh command evidence

| Command | Result |
|---|---|
| `git diff --check origin/develop` | exit 0 |
| `gofmt -l examples/graph-abuse-cluster` | empty |
| `go test -count=1 ./examples/graph-abuse-cluster/...` | exit 0; example 16.288s, internal package 0.848s |
| `go test -race -count=1 ./examples/graph-abuse-cluster/...` | exit 0; example 17.638s, internal package 1.426s |
| `go test -p 1 -count=1 ./examples/graph-abuse-cluster -run '^TestGraphAbuseClusterWithNeo4j$'` | exit 0; 15.878s |
| `go test -p 1 -count=1 ./...` | fresh rerun exit 0, including graph and transactional outbox packages |
| `make fmt-check` | exit 0 |
| `make tidy-check` | exit 0 |
| `make vet` | exit 0 |
| `make lint` | exit 0, `0 issues.` |
| `make test` | target이 `go test -p 1 -count=1 ./...`를 실행함을 확인; clean full sequential run exit 0, 이후 Colima host-port 오매핑 별도 기록 |
| `make race` | target이 `go test -p 1 -race -count=1 ./...`를 실행함을 확인; graph focused exit 0, 기존 SQL package의 Colima 오매핑 별도 기록 |
| `make ci` | fmt/tidy/vet/lint 및 변경 package는 통과; local Colima long-run은 unrelated host-port 오매핑으로 exit 2, clean GitHub gate 필수 |

`make lint`의 첫 실행은 exported declaration 주석 두 건, test helper context
인자 순서, literal nil-context test를 발견했다. 동작을 바꾸지 않는 최소 수정 후
focused normal/race, vet, lint와 전체 gate를 다시 실행했다.

전체 sequential test의 첫 실행에서는 변경되지 않은
`transactional-outbox-publisher`가 PostgreSQL `unexpected EOF`와 Redis port의
`HTTP/1.1 400 Bad Request`로 실패했다. branch diff는 이 package나 Testcontainers
core version을 바꾸지 않았다. 실패한 두 test만 같은 순서로 격리하면 3.295s에
통과했고, package 전체도 다섯 container pair를 연속 시작/정리하며 7.953s에
통과했다. 이어진 full sequential test, `make test`, `make race`, `make ci`가 모두
통과했다.

review에서 repository 규칙과 달리 `make test`/`make race`가 package를 병렬로
실행할 수 있다는 P1을 발견해 두 target에 `-p 1`을 고정했다. 변경 뒤 첫
`make test`에서는 변경되지 않은 `shared-redis-bloom-admission`의 Redis port가
다시 `HTTP/1.1 400 Bad Request`를 반환했다. 실패 test는 0.901s, package 전체는
4.700s에 격리 통과했다. 다음 long-run race에서는 세 기존 PostgreSQL package가
연속 connection reset에 걸렸지만 각각의 실패 test를 `-race`로 격리하면
2.559s, 2.423s, 2.468s에 통과했다. 당시 같은 Docker daemon에서 다른 repository의
20회 SQL stress test가 실행 중이었다. 그 실행 종료 뒤에도 별도 Redis package가
같은 `HTTP 400`을 한 번 반환해, clean GitHub runner를 최종 release gate로
지정했다. 두 경우 모두 제품 코드 변경 없이 Colima 동적 host-port mapping
혼선으로 분류했으며 첫 실패를 retry-only pass로 숨기지 않았다.

## Performance, stability, and security evidence

- 정상 DB path는 connectivity 뒤 `ExecuteWrite` 한 번, `ReadVertices` 한 번,
  `ReadEdges` 한 번이다. per-node query는 없다.
- fixture와 loaded graph는 각각 256 vertices, 1024 edges로 제한된다. 분석은
  recursion이 아닌 queue index 기반 iterative traversal이며 buffer도 이 경계 안이다.
- reset/delete와 seed는 같은 managed write에 있고 `fixture_id`가 모든 match와
  parameter에 포함된다. raw fixture value를 Cypher text에 보간하지 않는다.
- backend `ElementID`는 topology join에만 사용하고 JSON에는 validated opaque
  fixture ID만 사용한다.
- config는 userinfo, path, query, fragment, whitespace, non-loopback host, 잘못된
  port를 driver 생성 전에 거부한다. `NoAuth`는 이 local boundary 안에서만 사용한다.
- terminal error는 stable stage/class만 출력한다. URI, Cypher, provider error,
  opaque ID는 노출하지 않는다.
- operation cancellation과 별개인 cleanup context로 client/driver를 닫은 뒤에만
  완성된 JSON을 stdout에 쓴다.

## Dependency and license evidence

| Module | Version | License | Role |
|---|---:|---|---|
| `github.com/neo4j/neo4j-go-driver/v6` | `v6.1.0` | Apache-2.0 | direct runtime driver |
| `github.com/testcontainers/testcontainers-go/modules/neo4j` | `v0.42.0` | MIT | direct integration fixture |
| `github.com/neo4j/neo4j-go-driver/v5` | `v5.18.0` | Apache-2.0 | Testcontainers transitive dependency |

세 license는 repository의 MIT license와 배포 호환된다. 새 graph algorithm,
logging, CLI framework, Memgraph dependency는 없다.

## Diagram evidence ledger

### References

- `/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/graph-graph-core-architecture-01.png`
- `/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/diagrams/best-practices/assets/sequence-workflow-sample.png`
- `/Users/debop/work/bluetape4k/bluetape-go-workshop/.worktrees/feat-issue-50-graph-abuse-cluster/docs/images/readme-diagrams/audited-order-workflow-outbox-architecture.png`
- `/Users/debop/work/bluetape4k/bluetape-go-workshop/.worktrees/feat-issue-50-graph-abuse-cluster/docs/images/readme-diagrams/audited-order-workflow-outbox-sequence.png`
- Neo4j icon: `/Users/debop/work/bluetape4k/bluetape4k-wiki/docs/icons/testcontainers/graphdb/neo4j.svg`

### Canonical assets

| Asset | Dimensions | SHA-256 |
|---|---:|---|
| `graph-abuse-cluster-architecture.svg` | 1600x1030 source | `3febefaa98c6971e0377e4d685434b3454b59576ef67fd96e0ebcdac2cb83358` |
| `graph-abuse-cluster-architecture.png` | 3200x2060 | `9949969559ffefe9bcb1be4f60c84054fbc56fd23f26cedd9c14623c51ed685d` |
| `graph-abuse-cluster-sequence.svg` | 1600x1920 source | `77d1df2e899537ae4b6f1ecbf11aa913aa211c8d45375919c2d4196e82dd6113` |
| `graph-abuse-cluster-sequence.png` | 3200x3840 | `8744500daf5deea05e059348b19d9e9515d0eac0249f83af57598ac708e3388f` |

Architecture audit는 markers=4, connectors=8, cards=8, intrusions=0,
crossings=0이다. Sequence audit는 markers=6, connectors=25, cards=6,
intrusions=0, crossings=0이며 message/pill badge/number가 각각 25개다.
두 자산 모두 diagonal geometry, endpoint, mixed-corner 실패가 0이고 sequence
style audit도 통과했다. CairoSVG scale-2 재렌더와 canonical PNG의 `cmp`는
일치한다.

원본 크기 PNG 눈검사에서 자동 감사 밖의 결함도 수정했다.

1. architecture의 짧은 연속 꺾임을 rounded bend와 25px terminal segment로
   바꿨다.
2. 유사 Neo4j glyph를 catalog의 정확한 24x24 path로 교체했다.
3. sequence의 25번 arrow가 activation top corner에 닿던 endpoint를 side entry로
   바꿨다.
4. phase title과 3번 call pill을 서로 다른 수평 영역으로 분리했다.
5. path용 `.amber` selector가 20/21 number text에도 stroke를 주던 class 충돌을
   `path.amber`로 제한했다.
6. 초기 message를 실제 `driver/client/store -> fixture -> connectivity` 순서로
   맞추고 machine-local `file://` font URL을 제거했다.
7. architecture의 persistence와 application analysis가 모두 녹색 계열이던
   문제를 persistence 녹색과 application analysis 보라색으로 분리하되, 같은 수준의
   관계임을 나타내도록 둘 다 실선으로 유지했다.

최종 눈검사에서는 두 connector 범주가 녹색/보라색으로 명확히 구분되고, 두 범주의
화살촉이 PNG 변환 뒤에도 올바른 방향·색상·크기로 유지되는지 확인했다.

최종 PNG에서 모든 화살촉의 송신/수신 방향, 꺾임 뒤 marker 여유, card/label
침범, 1..25 call-number 가독성을 다시 확인했다.

## Six-lens final review

| Lens | Result | Evidence focus |
|---|---|---|
| Performance | P0=0, P1=0, P2=0 | fixed DB path, bounded data, iterative traversal, deterministic sort |
| Stability | P0=0, P1=0, P2=0 | atomic replace, cancellation, idempotence, cleanup, real backend |
| Security | P0=0, P1=0, P2=0 | strict loopback NoAuth, parameters, redaction, scoped delete |
| Operator/Ops | P0=0, P1=0, P2=0 | deadlines, close-before-output, exit behavior, runnable commands |
| Developer/API | P0=0, P1=0, P2=0 | narrow fixture/store/workflow seams, stable sentinels, no reusable-library drift |
| User/caller | P0=0, P1=0, P2=0 | exact bilingual lesson/output, caveats, navigation, inspected diagrams |

Code review graph가 29 changed files는 인식했지만 indexed function/flow를 0으로
반환해 structural evidence로 사용할 수 없었다. 따라서 source, tests, spec/plan,
README, rendered assets를 직접 대조했다.

첫 six-lens pass는 Testcontainers package serialization P1 한 건과 sequence의
초기화 순서 및 absolute font URL P2 두 건을 발견했다. Makefile을 실행 가능한
직렬 gate로 바꾸고 sequence를 runtime과 portable SVG 계약에 맞춘 뒤, 관련
tests와 diagram full audit/원본 눈검사를 반복해 최종 P0/P1/P2를 모두 0으로
닫았다.

## Residual boundaries

- fixed namespace를 공유하는 concurrent CLI run은 지원하지 않는다.
- fixture 전체를 memory에 모으는 교육용 예제이며 production-scale traversal이나
  fraud accuracy를 주장하지 않는다.
- auth, TLS, routing, authorization, retention, production deployment는 후속 범위다.
- Memgraph compatibility와 reusable algorithm은 `bluetape-go`의 별도 issue가
  필요하다.
