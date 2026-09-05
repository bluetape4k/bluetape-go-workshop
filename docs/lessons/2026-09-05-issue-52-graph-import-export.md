# Issue #52 graph import/export lesson

## 맥락

작은 named partner account/device graph를 NDJSON와 GraphML 사이에서 교환하려면
format parser의 성공만 확인해서는 부족하다. 입력 크기, record 수, scalar 표현,
edge 방향과 domain label을 같은 경계에서 확인하고, 두 format의 결과를 사람이
읽을 수 있는 기준 데이터로 비교해야 한다.

## 결정

- 기본 경로는 released `graph/graphio` NDJSON로 두고, named partner XML
  interoperability가 필요할 때만 `graph/graphio/graphml`의 directed subset을
  선택한다. GraphML 전체 생태계 호환성을 약속하지 않는다.
- import는 전체 입력을 `MaxInputBytes + 1`까지만 읽어 초과 여부를 확인한 뒤
  parser에 전달한다. 기본 상한은 전체 64 KiB, line/record 각 16 KiB, 총
  64 records다. `math.MaxInt64`는 `+1` 오버플로를 일으키므로 옵션 단계에서
  거부한다.
- vertex와 edge를 함께 본 record ID는 unique해야 한다. export와 기준 데이터는
  vertex/edge 내부에서 각각 ID 오름차순으로 정렬해 입력 순서에 의존하지
  않게 한다.
- JSON의 안전한 정수 범위는 `int64`로 정규화하고, GraphML typed scalar와
  boolean/string 값을 portable map으로 보존한다. GraphML의 `double`은 값이
  정수여도 `float64`로 유지한다. 배열·map·null·non-finite number는 거부한다.
- `partner` property와 `Account -> Device` 방향을 application domain invariant로
  검증한다. parser가 이해하지 못하는 key, nested graph, XML directive/extension,
  unsupported label은 부분 결과를 publish하지 않고 fail-closed한다.
- report에는 format별 summary와 ID 순서의 정규화된 그래프 기준 데이터를 함께 넣고,
  runtime duration은 제외해 재실행 결과를 비교할 수 있게 한다.

## 결과

`risk-partner-acme-v1` fixture는 `acme-payments` partner의 vertex 4개와 directed
edge 3개로 고정했다. 두 format의 round-trip은 모두 `vertices=4`, `edges=3`,
`accounts=2`, `devices=2`, `uses_device_edges=3`, `directed=true`를 만들고,
`account-a -> device-01` 같은 방향과 scalar type을 유지한다. CLI는 두 run의
기준 데이터가 같으면 `equivalent=true`를 출력한다.

## 검증

| 검증 | 결과 |
|---|---|
| NDJSON/GraphML focused test | PASS |
| focused race test | PASS |
| `go vet`와 `make tidy-check fmt-check vet lint` | PASS |
| 전체 `make test`와 `make race` | PASS |
| `go run ./examples/graph-import-export` | PASS; 두 format summary와 정렬된 기준 데이터 출력 |
| bilingual/root documentation parity | PASS; 명령, limit, unsupported scope, 방향과 기준 데이터 발췌 확인 |
| Korean terminology audit | PASS; 새 문서 finding 0건, 루트의 기존 cache `snapshot` 9건은 범위 밖 예외 |

## 발견한 누락과 수리

- 처음에는 `MaxInputBytes`에 `math.MaxInt64`를 넣으면 `max+1`이 overflow할
  수 있었다. 옵션 검증과 회귀 테스트를 추가해 fail-closed로 바꿨다.
- 알 수 없는 format을 오류 문자열에 그대로 넣으면 caller 입력이 진단
  출력으로 반사된다. stable `ErrInvalidFormat`만 반환하고 회귀 테스트로
  untrusted value가 포함되지 않음을 확인했다.
- summary만 문서화하면 정렬과 방향 보존을 확인하기 어렵다. 두 locale README에
  `account-a`, `use-001`, `from`, `to`를 포함한 정규화 결과 발췌를
  추가하고 documentation parity test가 이를 검사한다.

## 향후 guard

이 예제에 CSV CLI, broad GraphML extension, graph database adapter, compression,
filesystem write, atomic replacement, production risk decision을 추가하려면
새 issue에서 scope와 ownership을 다시 정한다. bounded default, scalar subset,
directed domain invariant, canonical order, redacted error, cancellation
경계를 바꾸는 수정에는 focused normal/race 회귀 테스트와 README 두 언어의
동시 갱신이 필요하다.

## 문서 점검

- SPW-01: PASS — audience, issue/버전, repository source, 명령과 비목표를
  현재 코드와 live issue에서 고정했다.
- SPW-02: PASS — lesson의 맥락, 결정, 결과, 검증, 누락·수리, future guard를
  모두 기록했다.
- SPW-03: PASS — Korean technical register와 issue/API identifier를 보존했다.
- SPW-04: PASS — 코드, 테스트, README, live issue의 limit·format·output을
  read-back했다.
- SPW-05: PASS — rendered Markdown heading/table/code fence와 링크를 다시
  읽고 `git diff --check`로 whitespace를 확인했다.
- KO-01~KO-06: PASS — 사실, 숫자, 명령, 링크, locale pair를 보존했다.
- KO-07: PASS — 새 Korean README에는 finding이 없고, 루트의 기존 9건은
  변경 범위 밖의 cache 용례로 기록했다.
