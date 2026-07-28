# Probabilistic Dedupe Admission Lesson 정리

Issue: #46

`examples/probabilistic-dedupe-admission`은
`github.com/bluetape4k/bluetape-go/probabilistic`를 사용하는 Bloom filter prefilter를
보여준다.

## 결정

webhook event admission scenario에는 `bluetape-go/probabilistic`의 in-memory
goroutine-safe Bloom filter를 사용한다. Bloom filter는 false positive를 만들 수 있으므로
예제는 repeat hit를 의도적으로 `duplicate`가 아니라 `probably_seen`으로 부른다.

## 경계

- prefilter key는 trim된 `event_id`다.
- 처음 본 event ID는 `definitely_new`로 admit한다.
- repeat 또는 false-positive hit는 `probably_seen`으로 드러낸다.
- 예제에는 durable store가 없으며, production dedupe가 중요한 data를 버리기 전에
  authoritative table, log, idempotency store가 필요함을 문서화한다.

## 후속 작업

Issue #78은 `probably_seen`을 durable workflow boundary와 여전히 reconcile해야 하는
prefilter signal로 취급해 이 예제를 checkout guard와 조합할 수 있다.
