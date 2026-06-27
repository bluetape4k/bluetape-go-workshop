# SQL Transaction Boundary 예제

이 예제는 SQL transaction이 어디에 있어야 하는지 보여줍니다. Service가
`sqlkit.WithTx`를 소유하고, repository는 전달받은 `*sql.Tx`로만 좁은 SQL
statement를 실행합니다. ORM, unit-of-work framework, 숨겨진 transaction manager를
도입하지 않습니다.

시나리오는 주문 생성입니다. Product row를 lock하고, stock을 차감하고, order
header와 order line을 insert합니다. Stock이 부족하거나, 뒤 payment check가
실패하거나, context가 cancel되면 모든 SQL 변경이 함께 rollback됩니다.

![SQL transaction boundary architecture](../../docs/images/readme-diagrams/sql-transaction-boundary-architecture.png)

## 시나리오

`Service.PlaceOrder`가 transaction boundary입니다.

1. Transaction을 열기 전에 request shape를 검증합니다.
2. Service layer에서 `sqlkit.WithTx(ctx, db, nil, fn)`를 시작합니다.
3. `select ... for update`로 product row를 정렬된 `product_id` 순서로 lock합니다.
4. 전달받은 `*sql.Tx`만 아는 repository를 통해 stock debit, order header,
   order line insert를 실행합니다.
5. Transaction function이 nil을 반환하면 commit하고, error를 반환하면
   rollback합니다.

![SQL transaction boundary sequence](../../docs/images/readme-diagrams/sql-transaction-boundary-sequence.png)

## Service가 Transaction을 소유하는 이유

| Layer | 책임 |
| --- | --- |
| Use case | Order placement 시작 시점과 context cancellation을 제공합니다. |
| Service | `sqlkit.WithTx`를 열고 repository call 순서를 정하며 commit/rollback을 결정합니다. |
| Repositories | Visible SQL을 만들고 전달받은 `*sql.Tx`로 실행합니다. |
| PostgreSQL | Row lock, constraint, 전체 write commit/rollback을 담당합니다. |

Repository method는 `BeginTx`, `Commit`, `Rollback`을 호출하지 않기 때문에 재사용할
수 있습니다. 이후 HTTP 예제는 같은 service를 호출할 수 있고, 다른 repository
예제는 operation boundary에 따라 `*sql.DB`나 `*sql.Tx`를 전달할 수 있습니다.

## 실행

Local transaction preview를 출력합니다.

```bash
go run ./examples/sql-transaction-boundary
```

출력은 JSON입니다. Lock, debit, order insert, line insert SQL shape와 commit /
rollback note를 포함합니다.

```json
{
  "transaction_boundary": "Service.PlaceOrder owns sqlkit.WithTx; repositories only use the *sql.Tx they receive.",
  "commit_path": [
    "validate the order request before opening a transaction",
    "begin sqlkit.WithTx at the service boundary",
    "lock product rows in sorted product_id order",
    "debit stock and insert order header plus lines",
    "commit only after payment and all SQL writes succeed"
  ]
}
```

PostgreSQL Testcontainers contract test를 실행합니다.

```bash
go test -count=1 ./examples/sql-transaction-boundary/...
```

이 예제의 race gate를 실행합니다.

```bash
go test -race -count=1 ./examples/sql-transaction-boundary/...
```

## 테스트가 증명하는 것

- Commit path는 stock debit과 order row insert를 함께 반영합니다.
- Stock 부족은 order를 만들거나 stock을 변경하지 않고 rollback됩니다.
- Stock debit 이후 payment rejection이 발생해도 debit과 insert가 함께
  rollback됩니다.
- Canceled context가 transaction boundary를 통해 전파됩니다.
- Preview SQL은 inspectable하게 유지하지만, correctness는 문자열 snapshot만이
  아니라 PostgreSQL 동작으로 증명합니다.

## Production Notes

- Retry는 transaction body 밖에 둡니다. Lock을 잡은 상태에서 retry하면 contention이
  커집니다.
- Concurrency test를 추가하기 전에 isolation level을 의도적으로 선택합니다.
- Row lock을 잡은 상태에서 외부 network call을 피합니다. 이 예제는 transaction
  lesson을 deterministic하게 유지하기 위해 flag로 payment rejection을 시뮬레이션합니다.
- Lock wait, rollback reason, commit latency metric을 기록합니다.
- Transaction boundary가 명확해진 뒤 outbox/event publication을 추가합니다. Commit
  전에 irreversible side effect를 publish하지 않습니다.
