# sql-access-strategy-decision

[English](README.md) | [한국어](README.ko.md)

`sqlkit`을 위한 local SQL access strategy 예제입니다.

이 예제는 의도적으로 web service보다 작게 만들었습니다. Handler, request DTO,
background job, migration pipeline을 붙이기 전에 팀이 먼저 정해야 하는 지점이
있습니다. SQL string은 누가 소유하는가, row cardinality는 어디에서 검증하는가,
transaction ceremony는 누가 책임지는가, generated query code와 schema migration
tooling은 runtime dependency 안으로 들어와야 하는가 같은 결정입니다.

코드는 같은 order-hold 흐름을 두 가지 runtime repository 형태로 실행합니다.

- direct `database/sql`: application이 raw SQL string, scanning, cardinality check,
  `BeginTx` / `Commit` ceremony를 직접 소유합니다.
- `sqlkit`: SQL text와 args는 계속 보이게 두되, 작은 statement builder,
  `QueryOne` / `QueryOptional`, `WithTx` rollback 동작은 bluetape-go helper에
  맡깁니다.

또한 `sqlc`, Jet, Atlas가 어디에 들어가야 하는지도 설명하지만, 이 예제의 runtime
dependency로는 추가하지 않습니다.

## 시나리오

Order service는 inventory와 payment check가 끝나기 전에 짧게 유지되는 customer
hold를 만듭니다. Service는 다음을 해야 합니다.

1. hold row를 insert합니다.
2. ID로 hold를 정확히 하나 읽습니다.
3. no-row와 too-many-row case를 감지합니다.
4. hold를 confirm하고 audit event를 같은 transaction 안에 남깁니다.
5. code review에서 SQL shape를 검사할 수 있게 유지합니다.

Schema는 일부러 작게 유지했습니다. `sql_strategy_holds`는 hold를 저장하고,
`sql_strategy_hold_events`는 audit trail을 저장합니다. 핵심은 schema design이
아니라, module에 필요한 SQL machinery의 크기를 고르는 것입니다.

## Architecture

![SQL access strategy decision architecture](../../docs/images/readme-diagrams/sql-access-strategy-decision-architecture.png)

Repository shape가 달라져도 application flow는 같습니다. Hold를 만들고, one-row
cardinality로 읽고, audit event와 함께 confirm합니다. Architecture diagram은 runtime
선택과 tooling boundary를 나눠 보여줍니다.

- direct `database/sql`은 query surface가 아주 작거나 driver behavior를 그대로
  다루는 것이 helper reuse보다 중요한 경우에 맞습니다.
- `sqlkit`은 visible SQL을 유지하면서 statement, row cardinality, transaction rollback
  helper를 공유하고 싶을 때 맞습니다.
- `sqlc` 또는 Jet는 generated-query boundary입니다. bluetape-go runtime dependency가
  아닙니다.
- Atlas는 migration planning, linting, apply workflow boundary입니다.

## Repository Sequence

![SQL access strategy decision sequence](../../docs/images/readme-diagrams/sql-access-strategy-decision-sequence.png)

Test는 두 repository shape를 PostgreSQL Testcontainer에 대해 실행합니다. Normal flow는
두 경로 모두 hold를 create, read, confirm할 수 있음을 증명합니다. 그 다음 `sqlkit`
중심 test가 ad hoc repository code에서 놓치기 쉬운 동작을 증명합니다.

1. Row가 없으면 `QueryOne`이 `sqlkit.ErrNoRows`를 반환합니다.
2. Singular business key가 여러 row를 반환하면 `QueryOptional`이
   `sqlkit.ErrTooManyRows`를 반환합니다.
3. Audit insert path가 `ErrAuditRejected`를 반환하면 `WithTx`가 status update를
   rollback합니다.
4. Canceled context가 repository 내부에서 사라지지 않고 driver까지 전달됩니다.
5. Statement snapshot은 SQL과 args를 test와 `go run` output에서 확인 가능하게
   유지합니다.

## Decision Table

| 선택 | 잘 맞는 경우 | 범위 밖에 둘 것 |
|---|---|---|
| direct `database/sql` | Query가 한두 개이고 raw SQL이나 driver-specific behavior를 직접 보여줘야 할 때. | Shared row cardinality helper, generated model, migration planning. |
| `sqlkit` | Visible SQL과 작은 builder, `QueryOne`, `QueryOptional`, `WithTx`가 필요할 때. | Schema metadata, ORM state, migration execution, generated package. |
| `sqlc` 또는 Jet | 안정적인 SQL surface가 커져 generated typed method가 hand-maintained scan code보다 싼 경우. | bluetape-go 내부 runtime dependency로 숨기지 말고 app에서 generated code를 review합니다. |
| Atlas | Schema diff, migration linting, migration plan/apply workflow가 핵심 문제일 때. | Repository runtime behavior와 request handling. |
| GORM, Bun, ent, goqu | Data model이 커진 뒤 product가 선택할 ORM/query-builder 결정. | 이 workshop 예제에서는 v0.7.0 `sqlkit` lesson을 흐리지 않기 위해 제외합니다. |

## 보여주는 것

- 작은 PostgreSQL-first statement를 만드는 `sqlkit.InsertInto`, `SelectFrom`, `Update`.
- Exactly-one-row read를 위한 `sqlkit.QueryOne`과 optional singular read를 위한
  `sqlkit.QueryOptional`.
- Original application error를 보존하면서 rollback하는 `sqlkit.WithTx`.
- Helper boundary가 마술처럼 보이지 않도록 direct `database/sql` contrast code를
  함께 둔 구조.
- Test, review, logging policy, onboarding에서 SQL text와 argument order를 확인할 수
  있는 statement snapshot.
- sqlc, Jet, Atlas, GORM, Bun, ent, goqu runtime dependency 없이 검증하는
  Testcontainers PostgreSQL coverage.

## 실행

Strategy report를 출력합니다.

```bash
go run ./examples/sql-access-strategy-decision
```

Output은 JSON입니다. direct `database/sql` statement, `sqlkit` statement, strategy
choice, production hardening note를 포함합니다.

```json
{
  "scenario": "Create a customer order hold, read it with one-row cardinality, and confirm it with an audit event in one transaction.",
  "direct_database_sql": [
    {
      "name": "direct.create_hold",
      "sql": "insert into sql_strategy_holds (id, customer, status) values ($1, $2, $3)"
    }
  ],
  "choices": [
    {
      "name": "sqlkit",
      "boundary": "runtime helper only; no schema metadata, migrations, generated models, or ORM state"
    }
  ]
}
```

실제 output에는 모든 statement와 args가 들어 있습니다. 위 snippet은 decision이 잘
보이도록 줄인 예시입니다.

## 테스트

이 test는 repository PostgreSQL Testcontainers fixture를 통해 Docker를 사용합니다.
다른 Testcontainers-backed suite와 마찬가지로 serial로 실행합니다.

```bash
go test -count=1 ./examples/sql-access-strategy-decision/...
go test -race -count=1 ./examples/sql-access-strategy-decision/...
```

Targeted test는 다음을 증명합니다.

- no-row와 too-many-row cardinality path;
- audit write 실패 시 transaction rollback;
- context cancellation propagation;
- inspectable SQL과 argument ordering.

## Boundary Notes

- Migration owner를 먼저 고릅니다. Repository helper는 불명확한 schema-change
  process를 대신 해결할 수 없습니다.
- Transaction ownership은 service boundary에 둡니다. Repository가 helper를 제공하더라도
  어떤 operation이 하나의 unit of work에 속하는지는 application이 결정해야 합니다.
- SQL shape와 decision metadata는 logging해도 되지만, customer data를 담을 수 있는 raw
  argument value는 노출하지 않습니다.
- Stable statement 수가 늘어나 generated method가 hand-maintained scan code보다 싸지는
  시점에 generated query code로 이동합니다.
- Schema evolution, review, apply workflow가 문제라면 Atlas 같은 migration tool
  boundary로 이동합니다.
