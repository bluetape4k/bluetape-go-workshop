# 교훈: Operations Report Policy 예제

## 변경

- 실행 가능한 Gin main package가 있는 `examples/operations-report-policy`를 추가했다.
- `workreport.Report` runtime timestamp를 생략하는 deterministic report DTO를 추가했다.
- 직접 `workreport.Aggregate` call을 사용해 `StopOnFailure`와 `ContinueOnFailure`를 보여 줬다.
- 보존된 failed attempt를 full success로 표시하지 않고 nested report로 retry evidence를 추가했다.
- `aborted`와 caller-defined reason을 사용해 skipped work representation을 추가했다.
- scenario, architecture, sequence section을 위한 English/Korean README 파일과 diagram asset을 추가했다.

## Guardrail

- `workreport` 예제는 정직하게 유지한다. 보존된 failed attempt가 있는 retry aggregate는 `partial` 상태여야 한다.
- report를 그대로 노출하지 말고 DTO로 project해서 API response를 deterministic하게 유지한다. `StartedAt` 또는 `EndedAt`를 노출하지 않는다.
- library가 first-class skipped status를 추가하기 전까지는 caller-skipped work에 reason이 있는 `aborted`를 사용한다.
- 이 focused 예제에 persistence, background worker, queue, observability dependency를 추가하지 않는다.

## 검증

- `bash scripts/generate-operations-report-policy-diagrams.sh`
- 다음 항목을 visual inspection했다.
  - `operations-report-policy-scenario.png`
  - `operations-report-policy-architecture.png`
  - `operations-report-policy-sequence.png`
  - `workshop-example-map.png`
- `go test -count=1 ./examples/operations-report-policy/...`
- `go test -race -count=1 ./examples/operations-report-policy/...`
- `go test -run '^$' ./examples/operations-report-policy`
