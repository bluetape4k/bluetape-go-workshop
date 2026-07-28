# Probabilistic Dedupe Admission 구현 계획

> **에이전트 작업자 참고:** 필수 하위 스킬: `superpowers:subagent-driven-development`(권장) 또는 `superpowers:executing-plans`를 사용해 이 계획을 작업 단위로 구현한다. 단계 추적에는 체크박스(`- [ ]`) 문법을 사용한다.

**목표:** `bluetape-go/probabilistic`을 사용해 event ID를 definitely new 또는 probably seen으로 사전 필터링하는 실행 가능한 Gin 예제를 만든다.

**아키텍처:** 예제에는 `probabilistic.BloomFilter[string]` 하나를 소유하고 안정적인 response DTO를 반환하며 Gin router를 노출하는 작은 `dedupe.Service`가 있다. Bloom filter는 admission prefilter일 뿐이며, 문서는 production dedupe에 여전히 durable authoritative storage가 필요하다는 점을 설명한다.

**기술 스택:** Go, Gin, `github.com/bluetape4k/bluetape-go/probabilistic`, 표준 `net/http` server timeout.

---

## 파일 구조

- `examples/probabilistic-dedupe-admission/main.go` 생성: `127.0.0.1:8099`에서 실행되는 loopback-only 서버.
- `examples/probabilistic-dedupe-admission/internal/dedupe/service.go` 생성: service, DTO, router, error mapping, stats projection.
- `examples/probabilistic-dedupe-admission/internal/dedupe/service_test.go` 생성: admit/probably-seen/invalid/stats 동작에 대한 service-level test.
- `examples/probabilistic-dedupe-admission/internal/dedupe/http_test.go` 생성: HTTP 성공, repeat event, 공개 오류, health에 대한 router test.
- `examples/probabilistic-dedupe-admission/README.md`와 `README.ko.md` 생성: 실행 지침, sample curl 호출, false-positive 및 durable-store 주의.
- root `README.md`와 `README.ko.md` 수정: examples table과 run section.
- `docs/lessons/2026-06-22-probabilistic-dedupe-admission.md` 생성: decision record와 follow-up context.
- `docs/review/2026-06-22-issue-46-probabilistic-dedupe-code-review.md` 생성: 구현 후 Step 6-R review artifact.

## 작업

### Task 1: Service TDD

- [ ] `examples/probabilistic-dedupe-admission/internal/dedupe/service_test.go`에 실패 테스트를 작성한다.
  - `TestServiceAdmitsDefinitelyNewEvent`
  - `TestServiceMarksRepeatedEventAsProbablySeen`
  - `TestServiceRejectsInvalidEventID`
  - `TestServiceStatsReflectAdmissions`
- [ ] `go test -count=1 ./examples/probabilistic-dedupe-admission/...`를 실행하고 정의되지 않은 service/type 때문에 실패하는지 확인한다.
- [ ] `Service`, `NewService`, `Admit`, `Stats`, DTO, sentinel error를 구현한다.
- [ ] `go test -count=1 ./examples/probabilistic-dedupe-admission/...`를 실행하고 service test가 통과하는지 확인한다.

### Task 2: HTTP TDD

- [ ] `examples/probabilistic-dedupe-admission/internal/dedupe/http_test.go`에 실패 테스트를 작성한다.
  - `TestRouterAdmitsAndThenMarksProbablySeen`
  - `TestRouterMapsInvalidRequest`
  - `TestHealthz`
- [ ] targeted test를 실행하고 router symbol이 없거나 실패하는지 확인한다.
- [ ] `NewRouter`, `/healthz`, `/events/admit`, `/filters/current`, max JSON body, 공개 오류 응답을 구현한다.
- [ ] targeted test를 실행하고 통과하는지 확인한다.

### Task 3: 실행 예제와 문서

- [ ] loopback-only `HTTP_ADDR`, server timeout, signal shutdown, 기본 port `8099`를 포함해 `main.go`를 추가한다.
- [ ] 실행 명령, first/repeat event curl 호출, stats endpoint, false-positive 주의, durable-store pairing을 담은 영어/한국어 예제 README를 추가한다.
- [ ] root 영어/한국어 README의 examples table과 run section을 갱신한다.
- [ ] `docs/lessons` 아래에 lesson note를 추가한다.

### Task 4: 검증과 Review

- [ ] 변경된 Go 파일에 `gofmt`를 실행한다.
- [ ] `go test -count=1 ./examples/probabilistic-dedupe-admission/...`를 실행한다.
- [ ] `go test -race -count=1 ./examples/probabilistic-dedupe-admission/...`를 실행한다.
- [ ] `go test -p 1 ./...`를 실행한다.
- [ ] `make ci`를 실행한다.
- [ ] `git diff --check`를 실행한다.
- [ ] Step 6-R local six-lane review를 실행하고 P0/P1을 수정한 뒤 review artifact를 `docs/review` 아래에 저장한다.

### Task 5: Commit and PR

- [ ] Lore trailer와 validation evidence를 포함해 commit한다.
- [ ] `feat/issue-46-probabilistic-dedupe`를 push한다.
- [ ] `develop` 대상 PR을 만들고 `Closes #46`, assignee `debop`, label `enhancement` 및 `examples`, milestone `0.6.0`을 연결한다.
- [ ] PR body가 `## DoD Status`로 끝나는지 확인한다.
- [ ] GitHub CI를 기다린다.
- [ ] CI가 통과하면 사용자 승인된 현재 continuation path에서 rebase merge하고, local `develop`을 sync하고, worktree를 제거한 뒤 local/remote feature branch를 삭제한다.

## Step 3-R Integrated Review

이 Codex surface에서는 native subagent spawning을 사용할 수 없으므로, full-feature reference contract를 사용해 여섯 review lane을 main-session의 독립 점검으로 실행했다.

| Priority | Area | Finding | 필요한 계획 수정 |
|---|---|---|---|
| P2 | Stability | 공유 filter state에는 변경 package race test가 필요하다. | targeted race 작업을 추가한다. 완료. |
| P2 | User | README가 Bloom filter를 authoritative storage처럼 암시하면 안 된다. | false-positive와 durable-store 문서 작업을 추가한다. 완료. |
| P2 | Developer | HTTP test는 service test뿐 아니라 repeat event를 직접 다뤄야 한다. | router duplicate-path 작업을 추가한다. 완료. |
| P3 | Operator | Stats endpoint는 production telemetry claim 없이 approximate value만 노출해야 한다. | stats projection만 추가한다. 완료. |

Final Step 3-R verdict: P0 = 0, P1 = 0.
