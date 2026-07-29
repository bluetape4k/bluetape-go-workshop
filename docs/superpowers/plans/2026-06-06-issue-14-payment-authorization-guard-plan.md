# Issue 14 Payment Authorization Guard 계획

## 범위

- 이슈: #14, `[v0.2.0] Add payment authorization circuit-breaker and bulkhead example`
- Spec:
  `docs/superpowers/specs/2026-06-06-issue-14-payment-authorization-guard-design.md`
- Worktree:
  `.worktrees/issue-14-payment-authorization-guard`
- 브랜치: `issue-14-payment-authorization-guard`

## 순서 제약

- 구현 전에 spec, spec review, plan을 먼저 commit한다.
- 모든 Go code, Go test, README example 작업에는 `$bluetape-go-patterns`를 적용한다.
- payment-specific diagram 작업에는 `$bluetape4k-diagram`을 적용한다.
- 기존 module resolution을 제외하고 `go.mod`는 바꾸지 않는다. 새 dependency를
  추가하면 안 된다.
- policy construction code를 쓰기 전에 Go module cache의 현재 `resilience` API를
  다시 확인한다.

## 작업

### 1. 계획 산출물 커밋

- complexity: low
- skill: `bluetape4k-full-feature`
- expected files:
  - `docs/superpowers/specs/2026-06-06-issue-14-payment-authorization-guard-design.md`
  - `docs/superpowers/reviews/2026-06-06-issue-14-payment-authorization-guard-spec-review.md`
  - `docs/superpowers/reviews/2026-06-06-issue-14-payment-authorization-guard-plan-review.md`
  - `docs/superpowers/plans/2026-06-06-issue-14-payment-authorization-guard-plan.md`
- action:
  - Step 3-R plan review를 먼저 실행한다.
  - Step 4 전에 Lore-format commit으로 planning artifact를 commit한다.
- verification:
  - `git status --short --branch`
  - `git log -1 --format=%B`

### 2. 얇은 Payment Authorization Guard 구현

- complexity: high
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/payment-authorization-guard/internal/paymentguard/authorize.go`
- action:
  - `Request`, `Authorization`, `Gateway`, `Options`, `Authorizer`를 추가한다.
  - zero-value default를 구현한다.
    - `FailureThreshold=2`
    - `OpenTimeout=250ms`
    - `MaxConcurrent=1`
  - 음수 option value를 거부한다.
  - 비어 있지 않은 `MerchantID`, 비어 있지 않은 `OrderID`, 양수
    `AmountCents`, non-nil authorizer, non-nil gateway를 검증한다.
  - `resilience.NewCircuitBreaker[Authorization]`와
    `resilience.NewBulkhead[Authorization]`.
  - open circuit가 bulkhead acquisition 및 gateway invocation보다 먼저 거부되도록
    policy를 `breaker, bulkhead` 순서로 실행한다.
- 다시 확인할 dependency assumption:
  - `resilience.CircuitBreakerOptions` required field.
  - `resilience.BulkheadOptions.Wait=false`의 immediate rejection 동작.
  - sentinel error `resilience.ErrCircuitOpen` 및
    `resilience.ErrBulkheadRejected`.
- rollback point:
  - policy construction이 얇게 유지될 수 없다면 helper abstraction을 추가하기 전에
    중단하고 spec을 수정한다.
- verification:
  - `gofmt -w examples/payment-authorization-guard/internal/paymentguard/authorize.go`
  - `go test -count=1 ./examples/payment-authorization-guard/...`

### 3. 집중 테스트 추가

- complexity: high
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/payment-authorization-guard/internal/paymentguard/authorize_test.go`
- action:
  - zero-value default의 success를 테스트한다.
  - 반복 gateway failure가 circuit를 여는지 테스트한다.
  - open circuit가 gateway invocation 전에 거부되고
    `resilience.ErrCircuitOpen`.
  - 첫 gateway call이 blocked된 상태에서 concurrent overflow와 즉시 두 번째 call
    rejection이 `resilience.ErrBulkheadRejected`와 matching되는지 테스트한다.
  - 최소한 다음 event에 대한 synchronous event capture를 테스트한다.
    `EventCircuitStateTransition`, `EventCircuitRejected`,
    `EventBulkheadAccepted`, and `EventBulkheadRejected`.
  - invalid request, nil gateway, nil authorizer, invalid negative option을
    테스트한다.
- concurrency note:
  - 이 모듈은 Go 모듈이므로 `MultithreadingTester`,
    `StructuredTaskScopeTester`, `SuspendedJobTester` 같은 Kotlin/JUnit helper는
    적용되지 않는다. channel, `sync.WaitGroup`, mutex-protected event capture,
    `go test -race`를 사용한다.
- verification:
  - `gofmt -w examples/payment-authorization-guard/internal/paymentguard/authorize_test.go`
  - `go test -count=1 ./examples/payment-authorization-guard/...`
  - `go test -race -count=1 ./examples/payment-authorization-guard/...`

### 4. Payment Authorization 다이어그램 자산 추가

- complexity: medium
- skill: `bluetape4k-diagram`
- expected files:
  - `docs/images/readme-diagrams/payment-authorization-guard-flow.dot`
  - `docs/images/readme-diagrams/payment-authorization-guard-flow.plain`
  - `docs/images/readme-diagrams/payment-authorization-guard-flow-graphviz.svg`
  - `docs/images/readme-diagrams/payment-authorization-guard-flow-graphviz.png`
  - `docs/images/readme-diagrams/payment-authorization-guard-flow.svg`
  - `docs/images/readme-diagrams/payment-authorization-guard-flow.png`
- action:
  - node/connector layout을 위한 Graphviz DOT/plain evidence를 만든다.
  - 가능한 경우 title/prominent label에는 `Architects Daughter`, detail label에는
    `Comic Mono`를 사용해 최종 README SVG/PNG pair를 만든다.
  - 렌더링된 PNG를 점검하고 overlapping label, detached connector, cropped text,
    blank output, one-color palette drift가 있으면 거부한다.
- verification:
  - `dot -Tplain docs/images/readme-diagrams/payment-authorization-guard-flow.dot -o docs/images/readme-diagrams/payment-authorization-guard-flow.plain`
  - `dot -Tsvg docs/images/readme-diagrams/payment-authorization-guard-flow.dot -o docs/images/readme-diagrams/payment-authorization-guard-flow-graphviz.svg`
  - `dot -Tpng docs/images/readme-diagrams/payment-authorization-guard-flow.dot -o docs/images/readme-diagrams/payment-authorization-guard-flow-graphviz.png`
  - `rsvg-convert docs/images/readme-diagrams/payment-authorization-guard-flow.svg -o docs/images/readme-diagrams/payment-authorization-guard-flow.png`
  - Inspect `docs/images/readme-diagrams/payment-authorization-guard-flow.png`.

### 5. English/Korean README pair 작성

- complexity: medium
- skill: `bluetape-go-patterns`
- expected files:
  - `examples/payment-authorization-guard/README.md`
  - `examples/payment-authorization-guard/README.ko.md`
- action:
  - `English | 한국어` language link를 추가한다.
  - payment authorization scenario, policy order, open-circuit rejection,
    bulkhead overflow, synchronous event, test command를 설명한다.
  - payment authorization diagram PNG를 embed한다.
  - README를 resilience API reference가 아니라 scenario-focused 문서로 유지한다.
- verification:
  - `rg -n "English \\| 한국어|payment-authorization-guard-flow.png|go test -count=1 ./examples/payment-authorization-guard/..." examples/payment-authorization-guard`

### 6. 루트 README 표 갱신

- complexity: low
- skill: `bluetape-go-patterns`
- expected files:
  - `README.md`
  - `README.ko.md`
- action:
  - 두 root table에 example row를 추가한다.
  - `English | 한국어` language-link wording을 사용한다.
  - v0.2.0 resilience example ordering을 기존 resilience example 근처에 유지한다.
- verification:
  - `rg -n "payment-authorization-guard|English \\| 한국어" README.md README.ko.md`

### 7. 검증 실행

- complexity: medium
- skill: `bluetape-go-patterns`
- expected files: 검증이 결함을 드러내지 않는 한 없음.
- action:
  - targeted test를 먼저 실행한다.
  - bulkhead overflow는 concurrency에 의존하므로 race test를 실행한다.
  - full repository test를 실행한다.
  - whitespace diff check를 실행한다.
  - 구성된 CI shortcut이 있고 실용적이면 실행한다.
- verification:
  - `go test -count=1 ./examples/payment-authorization-guard/...`
  - `go test -race -count=1 ./examples/payment-authorization-guard/...`
  - `go test ./...`
  - `git diff --check`
  - 외부 credential 없이 사용할 수 있고 정의되어 있으면 `make ci`.

### 8. Review, Commit, PR

- complexity: medium
- skill: `bluetape4k-full-feature`
- expected files:
  - Step 6-R review artifact under `docs/superpowers/reviews/`
  - PR branch commit(s)
- action:
  - 남은 `bluetape4k-full-feature` gate를 순서대로 실행한다.
    - Step 4: code/docs/diagram task 구현.
    - Step 4-T: test 갱신 및 targeted validation 실행.
    - Step 5: verifier checklist 기준 self-review.
    - Step 6: 최종 local validation.
    - Step 6-R: 7-tier code review와 `P0=0 P1=0` 수렴.
    - Step 7: Lore-format trailer로 implementation commit.
    - Step 7-P: PR open 및 PR body 검증.
    - Step 7-R: PR review/comment gate.
    - Step 8: GitHub CI gate.
    - Step 9: evidence 포함 최종 보고.
  - Lore-format trailer로 implementation을 commit한다.
  - PR body 끝에 Step DoD table을 두고 issue #14용 PR을 연다.
  - PR body verification, PR review/comment gate, GitHub CI gate를 실행한다.
- verification:
  - `git log --oneline --decorate -3`
  - `gh pr view --json number,title,url,headRefName,baseRefName`
  - `gh pr checks --watch` 또는 동등한 CI status command.

## 인수 조건 매핑

| Issue #14 인수 조건 | Plan task |
| --- | --- |
| `go test -count=1 ./examples/payment-authorization-guard/...` | Tasks 2, 3, 7 |
| 반복 gateway failure가 circuit를 열기 | Task 3 |
| open circuit가 gateway call 전에 거부하기 | Tasks 2, 3 |
| concurrent overflow가 `ErrBulkheadRejected` 반환하기 | Task 3 |
| circuit transition 및 rejection event | Task 3 |
| 새 dependency 없음 | Tasks 2, 7 |
| 루트 README 및 Korean README table | Task 6 |

## 중단 조건

Step 9가 다음 증거를 가진 뒤에만 중단한다.

- local validation command,
- Step 6-R `P0=0 P1=0`,
- PR creation,
- PR body verification,
- PR review/comment gate,
- GitHub CI status.

Merge is not part of this plan unless the user explicitly asks to merge after
the PR is ready.
