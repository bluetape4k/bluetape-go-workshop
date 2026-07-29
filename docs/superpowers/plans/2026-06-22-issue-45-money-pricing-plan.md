# Issue #45 계획: Money와 Rule 기반 Pricing 예제

Spec: `docs/superpowers/specs/2026-06-22-issue-45-money-pricing-design.md`

## 계획

1. `examples/money-rule-pricing/internal/moneypricing`에 실패 테스트를 추가한다.
   - 반올림된 line total과 discount total
   - item/cart currency 혼합 거부
   - 허용되는 VIP 및 `SAVE10` discount
   - 지원하지 않는 coupon 거부
   - subtotal을 초과하는 fixed discount 거부
2. `POST /quotes`와 공개 오류 매핑에 대한 실패 HTTP 테스트를 추가한다.
3. `moneypricing` domain type, quote service, money DTO helper, 결정적 rule decision을 구현한다.
4. `/healthz`와 `/quotes`, JSON body cap, 공개 오류 응답, loopback-only `main.go` binding을 포함한 Gin router를 구현한다.
5. 실행 명령, 예상 동작, 명확한 no-`float64` money 주의를 담은 영어/한국어 예제 README를 추가한다.
6. root 영어/한국어 README에 새 예제 항목을 추가한다.
7. `docs/lessons/2026-06-22-money-rule-pricing.md`를 추가한다.
8. `go mod tidy`를 실행해 필요한 `bluetape-go/money` 전이 checksum을 추가한다.
9. 다음으로 검증한다.
   - 변경된 Go 파일의 `gofmt`
   - `go test -count=1 ./examples/money-rule-pricing/...`,
   - `go test -race -count=1 ./examples/money-rule-pricing/...`,
   - `go test -p 1 ./...`,
   - `make fmt-check`,
   - `make tidy-check`,
   - `make vet`,
   - `make lint`,
   - `git diff --check`.
10. code review를 실행하고 finding을 `docs/review` 아래에 기록한다. P0/P1 이슈를 수정한 뒤 commit, push하고 issue #45와 연결된 PR을 열며 milestone과 label은 issue에서 상속한다.

## 작업 매핑

| Spec 요구사항 | 계획 작업 |
|---|---|
| 실행 가능한 Gin API | 2, 4, 5 |
| `bluetape-go/money` 사용 | 1, 3, 8 |
| `float64` money arithmetic 금지 | 1, 3, 5 |
| Currency mismatch 거부 | 1, 2, 3, 4 |
| 반올림된 line total과 discount total | 1, 3 |
| 유효한 discount rule | 1, 3 |
| 거부되는 rule 경로 | 1, 2, 3 |
| 영어/한국어 문서 | 5, 6, 7 |
| 검증 gate | 9, 10 |

## Step 3-R Integrated Review

이 Codex surface에서는 native subagent spawning을 사용할 수 없으므로, full-feature reference contract를 사용해 여섯 review lane을 main-session의 독립 점검으로 실행했다.

| Priority | Area | Finding | 필요한 계획 수정 |
|---|---|---|---|
| P2 | Stability | HTTP와 service test가 rejected rule은 non-fatal이고 invalid money/currency error는 fatal임을 모두 증명해야 한다. | service와 HTTP 테스트 작업을 분리해 추가한다. 완료. |
| P2 | Developer | `bluetape-go/money` import 시 `govalues` dependency sum이 추가될 수 있다. | `go mod tidy` 작업을 추가한다. 완료. |
| P3 | Operator | 이 예제는 외부 resource가 없으므로 runtime stability에는 race test와 package test로 충분하다. | targeted verification과 repo gate를 유지한다. 완료. |

Final Step 3-R verdict: P0 = 0, P1 = 0.
