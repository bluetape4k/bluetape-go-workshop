# Issue #50 graph abuse cluster lessons

## Backend와 policy를 분리한다

Neo4j는 fixed fixture namespace의 저장과 조회를 담당하고, Go는 graph 의미를
담당한다. connected component, shared evidence, 5/3/1 score, cluster order를
Cypher로 숨기지 않으니 작은 fixture로도 정책을 정확히 unit test할 수 있다.
workshop에서 발견한 reusable abstraction은 예제 안에 키우지 말고
`bluetape-go`의 별도 issue로 보낸다.

## ElementID와 opaque ID는 역할이 다르다

Neo4j `ElementID`는 한 번 읽은 graph의 vertex/edge endpoint를 연결하는 backend
identity다. domain report와 deterministic order는 fixture의 validated
`opaque_id`를 사용한다. 둘을 섞으면 재기동이나 backend별 ID 차이가 사용자
출력에 새어 나온다. integration test는 backend ID가 opaque ID와 다르면서도
edge endpoint가 정상 연결되는지 확인해야 한다.

## reset과 seed는 하나의 write다

scoped delete를 먼저 성공시키고 seed를 따로 실행하면 두 번째 단계 실패 시
빈 namespace가 남는다. `OPTIONAL MATCH`와 parameterized `FOREACH/UNWIND`를 한
managed write에 넣어 empty namespace, repeat run, rollback을 같은 계약으로
만들었다. delete에는 항상 exact `fixture_id`가 있어야 하며 unscoped cleanup은
금지한다.

## 경계는 max가 아니라 max+1로 읽는다

`LIMIT 256`은 257번째 record 존재 여부를 숨긴다. vertices는 257, edges는 1025를
요청하고 결과가 256/1024를 넘으면 typed size error를 반환한다. exact limit
acceptance와 max+1 rejection을 write-side fixture와 read-side loaded graph에서
각각 증명해야 한다.

## NoAuth는 더 엄격한 local boundary를 요구한다

`NoAuth`를 쓸 때 URI parser를 일반 connection parser처럼 만들면 안 된다. exact
`bolt` scheme, numeric port, `localhost`/loopback IP만 허용하고 userinfo, path,
query, fragment, whitespace, routing/TLS scheme을 driver 생성 전에 거부한다.
오류에는 URI나 provider cause를 출력하지 않고 stable stage/class만 남긴다.

## 성공 output은 cleanup 뒤에 쓴다

JSON을 memory에 완전히 encode한 뒤 operation context와 독립된 fresh cleanup
context로 client/driver를 닫는다. close가 성공한 뒤에만 checked full-write로
stdout에 복사한다. defer는 실패 경로의 안전망이고, 성공 경로의 순서를 대신하지
않는다. close attempt flag는 호출 전에 세워 재시도나 double close를 막는다.

## Testcontainers 실패는 retry로 덮지 않는다

부분 시작 시 이미 열린 container를 즉시 정리하고, test cleanup은 namespace
삭제가 container 종료보다 먼저 실행되도록 LIFO를 고려한다. cancellation test는
호출 반환 뒤 잠시 기다린 후 late mutation이 없는지도 확인한다.

이번 full sequential run은 변경되지 않은 다른 package에서 PostgreSQL EOF와
Redis port의 HTTP 응답으로 한 번 실패했다. 실패 test 격리, package 전체,
full sequential, `make test`, `make race`, `make ci` 순서로 원인을 분리했다.
정확한 protocol mismatch와 이후 반복 통과가 Colima host-port mapping의 일시적
혼선을 가리켰으므로 제품 코드에 무관한 retry를 넣지 않았다. process handle과
fresh exit code가 없는 실행은 증거로 쓰지 않는다.

명령을 사람이 `go test -p 1`로 실행하는 것만으로는 repository 계약이 되지
않는다. container-backed package가 Docker 자원을 공유하는 이 workshop은
`make test`와 `make race` 자체에 `-p 1`을 넣어 CI와 로컬의 package-level
serialization을 동일하게 강제해야 한다. 직렬 실행 중에도 다른 process가 같은
Docker daemon을 쓰면 host-port 혼선은 생길 수 있으므로 실패 원인 격리는 여전히
필요하다.

## lint는 마지막에 처음 실행하지 않는다

focused tests와 vet가 통과해도 revive/staticcheck 계약은 남을 수 있다. 이번에는
Task 9 첫 `make lint`에서 exported block comment, context-first helper,
literal nil-context call을 발견했다. 새 package surface를 만든 직후 targeted
test와 함께 repository lint를 실행하면 최종 gate에서의 되돌림을 줄일 수 있다.

## 다이어그램 자동 감사 뒤에 반드시 눈검사한다

자동 감사가 통과해도 다음 결함은 남을 수 있다.

- SVG에서 올바른 marker가 PNG에서 원하는 방향으로 보이는지;
- rounded bend 뒤 terminal segment가 arrowhead보다 충분히 긴지;
- endpoint가 activation corner가 아니라 의도한 side에 닿는지;
- 긴 phase title과 call pill이 같은 y-band에서 겹치지 않는지;
- generic CSS class가 text와 path 양쪽에 적용되지 않는지;
- catalog icon path가 비슷한 모양이 아니라 exact source인지.
- sequence의 call 순서가 실제 resource 생성/검증 순서와 같은지;
- SVG가 특정 machine의 absolute `file://` font URL에 의존하지 않는지.

이번 sequence에서 path color class `.amber`가 call-number text에도 stroke를 줘
20/21이 굵은 glyph처럼 보였다. selector를 `path.amber`로 좁혀 해결했다. DOM의
연속 번호와 style audit만으로는 이 결함을 찾을 수 없었다. 마지막 좌표/CSS 변경
후 CairoSVG scale-2 렌더, deterministic `cmp`, original-size PNG 검사를 다시
수행해야 한다.

## roadmap body 변경은 fail closed로 다룬다

#27 같은 roadmap body를 자동 갱신할 때는 현재 body를 먼저 snapshot하고,
body-file로 한 번만 적용한 다음 live fetch로 exact marker와 기존 내용을
검증한다. update나 검증이 실패하면 snapshot을 즉시 복원하고 하위 issue 작업을
계속하지 않는다. metadata mutation의 부분 성공을 정상 상태로 해석하지 않는다.
