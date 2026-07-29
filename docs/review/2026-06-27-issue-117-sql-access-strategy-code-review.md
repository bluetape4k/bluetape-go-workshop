# Code review: issue #117 SQL access strategy decision

## 범위

- 새 runnable example: `examples/sql-access-strategy-decision`
- architecture와 repository sequence를 위한 새 README diagram
- root README navigation과 focused run instruction
- SQL access strategy boundary selection lesson note

## 발견 사항

local review pass에서 P0/P1 finding은 없다.

## 점검

- direct `database/sql` path는 raw SQL, manual scan/cardinality behavior, explicit
  `BeginTx` / `Commit` transaction ceremony를 소유한다.
- `sqlkit` path는 SQL text나 argument ordering을 숨기지 않고 statement builder, `QueryOne`,
  `QueryOptional`, `WithTx`를 사용한다.
- generated query tool과 migration tooling은 runtime dependency로 추가하지 않고 boundary로
  문서화한다.
- Testcontainers PostgreSQL test는 no-row, too-many-row, rollback, context cancellation,
  statement snapshot behavior를 다룬다.
- README diagram은 PNG로 render되며 architecture choice와 runtime repository sequence를
  분리한다.

## 검증 Evidence

- `go run ./examples/sql-access-strategy-decision`
- `go test -count=1 ./examples/sql-access-strategy-decision/...`
- `go test -race -count=1 ./examples/sql-access-strategy-decision/...`
- `make ci`
- SVG XML parse plus CairoSVG render for
  `sql-access-strategy-decision-architecture.svg` and
  `sql-access-strategy-decision-sequence.svg`
- root README files와 새 example README pair에 대한 local link/image existence check
- `git diff --check`

## 잔여 Risk

예제는 service-layer HTTP behavior, isolation-level selection, retry policy, migration
execution, generated query package를 모델링하지 않는다. 이들은 downstream choice로 문서화되어
있으며, 이 예제는 SQL access decision에 집중한다.

local diagram skill이 참조한 optional diagram geometry 및 endpoint audit helper script가 설치된
경로에 없었기 때문에 diagram gate는 XML parsing, CairoSVG rendering, marker inspection,
full-size PNG inspection을 사용했다.

## P0/P1 Gate

P0=0 P1=0
