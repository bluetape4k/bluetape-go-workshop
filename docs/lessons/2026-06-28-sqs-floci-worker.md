# SQS Floci Worker 예제 Lesson

## 맥락

Issue #60은 v0.7.0 AWS/Floci workshop track에 SQS worker step을 추가한다. reader가 이해해야
하는 것은 broad queue framework가 아니라 SQS 주변 application boundary다. 여기에는 message
shape, idempotency metadata, receive policy, acknowledgement, retry visibility가 포함된다.

## 결정

예제는 AWS SDK v2 client를 caller-owned로 유지하고 `Enqueue`와 `PollOnce`가 있는 작은
`TaskQueue` boundary를 노출한다. `PollOnce`는 message 하나를 받고 application handler를
호출하며, 성공 뒤에만 delete하고 실패 뒤에는 visibility를 변경해 SQS가 같은 idempotent task를
다시 전달할 수 있게 한다.

## 기각한 선택

- long-running goroutine worker. lesson이 SQS delivery semantics가 아니라 lifecycle과
  shutdown으로 바뀐다.
- real AWS smoke test. cloud credential, IAM state, account cleanup이 필요하다.
- deterministic test path의 DLQ implementation. DLQ/redrive policy는 production
  infrastructure concern이므로, 예제는 이를 문서화하되 local contract는 ack와 retry visibility에
  집중한다.

## 검증 형태

- fake-client test는 send body/attribute, receive policy, success delete, failure
  visibility change, invalid body retry, no-message handling, send 전 cancellation을
  증명한다.
- opt-in Floci smoke test는 real AWS credential 없이 local SQS round trip,
  delete-after-success, retry-visible failure를 증명한다.
- README diagram은 static ownership과 success/failure operation sequence를 분리해
  at-least-once 및 idempotency rule이 보이게 한다.
