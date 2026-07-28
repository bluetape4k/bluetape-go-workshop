# Issue #40 계획: Operations Report Policy 예제

## 목표

deterministic `workreport` output과 `StopOnFailure`/`ContinueOnFailure`의 동작
차이를 보여 주는 Gin API인 `examples/operations-report-policy`를 만든다.

## 구현 단계

1. 예제 package skeleton을 추가한다.
   - `examples/operations-report-policy/main.go`
   - `examples/operations-report-policy/internal/operations/server.go`
   - `examples/operations-report-policy/internal/operations/server_test.go`
2. `run_id`, `policy`, `products_valid`,
   `retry_partner_notification`, and `skip_search_index`.
   필드를 가진 request model을 구현한다.
3. report construction을 구현한다.
   - run을 만들기 전에 `ctx.Err()`를 확인하고, cancelled 상태이면
     `workreport.Cancelled("operations-run", err)` when cancelled
     를 반환한다.
   - deterministic child report를 만든다.
   - 요청된 `workreport.FailurePolicy`로 aggregate한다.
4. stable projection을 구현한다.
   - timestamp 없는 report node DTO
   - root 및 모든 descendant에 대한 summary count
   - completed, partial, failed, aborted, cancelled용 HTTP status mapper
5. endpoint behavior, failure policy, retry evidence, skip mapping, invalid input,
   cancellation, summary count용 focused test를 추가한다.
6. README.md와 README.ko.md를 추가한다.
   - scenario
   - report fields
   - failure policy behavior
   - production hardening gaps
   - architecture and sequence diagram sections
7. `scripts/generate-operations-report-policy-diagrams.sh`를 추가하고 PNG 및 SVG
   asset을 생성한다.
8. root README.md와 README.ko.md의 example table, run section, v0.4.0 roadmap
   wording을 갱신한다.
9. lesson note와 Step 6-R code review artifact를 추가한다.
10. PR을 열기 전에 targeted test, race test, diagram inspection, diff check,
    repository CI gate로 검증한다.

## 검증 명령

```bash
bash scripts/generate-operations-report-policy-diagrams.sh
go test -count=1 ./examples/operations-report-policy/...
go test -race -count=1 ./examples/operations-report-policy/...
go test -run '^$' ./examples/operations-report-policy
git diff --check
make ci
```

## PR 산출물

- planning commit 이후 implementation commit
- `## DoD Status`로 끝나는 English PR body
- test 및 CI check evidence
- 명시 요청 전 merge 금지
