# Issue #60 SQS Floci Worker Code Review

## 범위

- Branch: `feat/issue-60-sqs-floci-worker`
- Baseline: `origin/develop`
- Changed area: `examples/sqs-floci-worker`, root README pair, SQS README diagram,
  `go.mod`

## 발견 사항

P0=0 P1=0

- P0 finding 없음: 예제는 real AWS credential이 필요 없고 Floci smoke coverage를
  `BLUETAPE_SQS_FLOCI_WORKER_SMOKE` 뒤에 opt-in으로 둔다.
- P1 finding 없음: public operation은 caller context를 받고, success delete는 handler completion
  이후에만 실행되며, failure는 delete하지 않고 retry를 위해 visibility를 변경한다. README는 SQS
  at-least-once delivery와 idempotency requirement를 설명한다.

## Evidence

- `go run ./examples/sqs-floci-worker`
- `go test -count=1 ./examples/sqs-floci-worker/...`
- `go test -race -count=1 ./examples/sqs-floci-worker/...`
- `xmllint --noout docs/images/readme-diagrams/sqs-floci-worker-architecture.svg docs/images/readme-diagrams/sqs-floci-worker-sequence.svg`
- both SVG asset에 대해 `~/.local/bin/cairosvg ... -s 2` 실행 후 rendered PNG inspection.

## 잔여 Risk

- smoke test는 Docker가 필요하므로 opt-in이다.
- production DLQ redrive policy, idempotency storage, long-running work를 위한 visibility
  extension, concurrency control, metric, alarm, IAM은 이 focused workshop example의 범위 밖이다.
- installed diagram skill이 참조한 diagram geometry/endpoint helper script는 local skill directory에
  없어서 validation은 XML parse, CairoSVG render, marker/icon scan, PNG inspection을 사용한다.
