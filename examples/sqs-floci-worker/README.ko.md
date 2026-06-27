# SQS Floci Worker 예제

이 예제는 fulfillment 작업을 위한 작은 SQS producer/consumer boundary를 보여줍니다.
AWS SDK v2 client는 caller-owned로 유지하고, application code는 payload shape,
message attribute, acknowledgement 규칙, retry visibility만 소유합니다. Floci는
smoke test를 위한 local SQS endpoint와 test credential만 제공합니다. Production
SQS delivery contract는 그대로입니다.

![SQS Floci worker architecture](../../docs/images/readme-diagrams/sqs-floci-worker-architecture.png)

## Scenario

Checkout flow가 fulfillment task를 enqueue합니다. Worker는 message 하나를 poll하고,
task를 decode한 뒤 application handler를 호출합니다. Handler가 성공하면 SQS message를
delete합니다. Handler가 실패하면 message를 delete하지 않고 visibility를 바꿔 같은
task가 retry 대상으로 다시 보이게 합니다.

| Operation | 예제가 소유하는 것 | 독자가 얻어야 할 점 |
|---|---|---|
| `Enqueue` | JSON body, `content-type`, `event-type`, `idempotency-key` attribute | SQS message에는 안전한 처리와 dedupe에 필요한 metadata가 있어야 합니다. |
| `PollOnce` | long polling, one-message receive, handler call, ack 또는 retry visibility | Delete가 acknowledgement boundary입니다. |
| Handler success | `DeleteMessage` | Application work가 성공한 뒤에만 acknowledge합니다. |
| Handler failure | `ChangeMessageVisibility` | Retry는 transaction이 아니라 visibility timeout으로 제어합니다. |

![SQS Floci worker sequence](../../docs/images/readme-diagrams/sqs-floci-worker-sequence.png)

## Delivery Semantics

SQS는 at-least-once입니다. 같은 message가 두 번 이상 전달될 수 있고, handler가 side
effect 일부를 끝낸 뒤 실패할 수도 있습니다. 따라서 production handler는 반드시
idempotent해야 합니다. 이 예제는 `IdempotencyKey`를 JSON body와 SQS message
attribute 양쪽에 담아, 실제 handler가 `tenant/order/step` 같은 domain key로 dedupe할
수 있게 합니다.

Worker는 의도적으로 message 하나만 poll합니다. 예제의 초점을 acknowledgement와 retry
behavior에 두기 위해서입니다. Production worker는 batch, parallel processing,
visibility extension, DLQ routing, metric을 추가할 수 있지만, 모두 같은 boundary
주변의 infrastructure concern입니다.

## Local Floci vs Real AWS

| Concern | Local Floci smoke | Real AWS deployment |
|---|---|---|
| Credential | `testcontainers/floci`의 test credential | IAM role 또는 workload identity |
| Endpoint | Local endpoint override | Normal regional SQS endpoint |
| Queue | Opt-in smoke test 중 생성 | Infrastructure code로 provision |
| Retry | 짧은 timeout의 `ChangeMessageVisibility` | Visibility timeout, redrive policy, DLQ, alarm |
| Idempotency | Payload와 attribute로 시연 | Handler state 또는 downstream idempotent API로 강제 |

## 실행

Local worker preview를 출력합니다.

```bash
go run ./examples/sqs-floci-worker
```

Deterministic worker test를 실행합니다.

```bash
go test -count=1 ./examples/sqs-floci-worker/...
```

Docker-backed Floci SQS smoke test를 opt-in으로 실행합니다.

```bash
BLUETAPE_SQS_FLOCI_WORKER_SMOKE=1 go test -run TestFlociSmoke -count=1 ./examples/sqs-floci-worker/...
```

## Boundary Notes

- 이 예제는 SQS framework가 아닙니다. Request shape, acknowledgement, retry
  visibility를 설명하는 scenario-shaped reference입니다.
- Real deployment에서는 DLQ redrive policy, handler idempotency storage, 긴 작업을
  위한 visibility extension, structured log, metric, alarm을 추가해야 합니다.
- Floci smoke test는 real AWS credential이나 account state 없이 local AWS SDK request
  compatibility를 증명합니다.
