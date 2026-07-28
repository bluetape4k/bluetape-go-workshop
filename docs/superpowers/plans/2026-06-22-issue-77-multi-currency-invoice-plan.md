# Multi-Currency Invoice Rule Evaluation 예제 구현 계획

> **에이전트 작업자 참고:** 작업 단위로 구현한다. 같은 branch에서 이 계획을 실행할 때 checkbox 상태를 최신으로 유지한다.

**목표:** 명시적인 money semantic과 local rule decision으로 multi-currency invoice를 평가하는 focused Gin 예제 `examples/multi-currency-invoice-rules`를 추가한다.

**아키텍처:** 예제 내부 전용 `internal/invoicerules` 패키지 하나가 request validation, money parsing, per-currency accumulator, rule decision, 공개 오류 매핑, Gin router를 소유한다. `main.go`는 loopback HTTP 서버 연결만 담당한다.

**기술 스택:** Go, Gin, `github.com/bluetape4k/bluetape-go/money`, 표준 라이브러리 `errors`, `net/http`, `sort`, `strconv`, `strings`. 새 직접 의존성은 추가하지 않는다.

## 제약

- 모든 amount/currency 처리에는 `bluetape-go/money`를 사용한다.
- rule은 예제 내부에 유지한다. 범용 rule engine은 추가하지 않는다.
- exchange-rate conversion이나 cross-currency total은 제공하지 않는다.
- 구현 전에 테스트를 먼저 작성한다.
- 문서는 영어/한국어 쌍을 유지하고 root navigation을 갱신한다.
- 공개 오류 allowlist를 보존하고 원본 parser diagnostic 누출을 피한다.

## 계획 파일

- `examples/multi-currency-invoice-rules/main.go`
- `examples/multi-currency-invoice-rules/main_test.go`
- `examples/multi-currency-invoice-rules/README.md`
- `examples/multi-currency-invoice-rules/README.ko.md`
- `examples/multi-currency-invoice-rules/internal/invoicerules/service.go`
- `examples/multi-currency-invoice-rules/internal/invoicerules/service_test.go`
- `examples/multi-currency-invoice-rules/internal/invoicerules/http_test.go`
- `README.md`
- `README.ko.md`
- `docs/lessons/2026-06-22-multi-currency-invoice-rules.md`
- `docs/review/2026-06-22-issue-77-multi-currency-invoice-code-review.md`

## 구현 작업

- [x] **A. 서비스 TDD red 테스트 [복잡도: 중간]**
  - 그룹화된 USD/EUR invoice total, VIP service discount eligibility, EU tax-like adjustment, JPY zero-minor-unit rounding, invalid currency rejection 테스트를 추가한다.
  - `go test -count=1 ./examples/multi-currency-invoice-rules/internal/invoicerules`를 실행하고 예상되는 compile/fail 출력을 TDD 증거로 남긴다.

- [x] **B. 서비스 구현 [복잡도: 중간]**
  - DTO, sentinel, `Service.Evaluate`, per-currency accumulator, `vip-service-discount`, `regional-vat`, money formatting, stable sorting, 공개 오류 매핑을 구현한다.
  - 집중 package test를 실행한다.

- [x] **C. HTTP와 main TDD/구현 [복잡도: 낮음]**
  - 성공, malformed JSON, invalid currency, `/healthz`에 대한 router test를 추가한다.
  - 기본 loopback 주소, 유효한 loopback override, non-loopback 거부, 제한된 server timeout에 대한 main test를 추가한다.
  - 기본값 `127.0.0.1:8100`으로 `main.go`를 구현한다.

- [x] **D. 문서 [복잡도: 중간]**
  - 시나리오, endpoint 표, 실행 명령, curl request, grouped total response, invalid currency 예시, production boundary를 담은 영어/한국어 예제 README를 추가한다.
  - root 영어/한국어 README 표와 실행 섹션을 갱신한다.
  - #45를 기반 예제로 연결하는 lesson note를 추가한다.

- [x] **E. 집중 검증 [복잡도: 중간]**
  - 다음을 실행한다.
    - `go test -count=1 ./examples/multi-currency-invoice-rules/...`
    - `go test -race -count=1 ./examples/multi-currency-invoice-rules/...`
    - `go run ./examples/multi-currency-invoice-rules`로 live smoke를 실행해 `/healthz`와 `/invoices/evaluate`를 확인한다.

- [x] **F. 전체 repository 검증 [복잡도: 높음]**
  - 다음을 실행한다.
    - `go test -p 1 ./...`
    - `make fmt-check`
    - `make tidy-check`
    - `make vet`
    - `make lint`
    - `GOFLAGS=-p=1 make ci`
    - `git diff --check`
  - 범위 안의 실패를 수정한다.

- [x] **G. Step 6-R code review와 수정 [복잡도: 중간]**
  - six-lane review와 security/trust-boundary review를 실행한다.
  - `docs/review/2026-06-22-issue-77-multi-currency-invoice-code-review.md`에 저장한다.
  - 모든 P0/P1 finding을 수정하고 영향받은 테스트를 다시 실행한다.

- [ ] **H. Commit, PR, merge, local sync [복잡도: 낮음]**
  - Lore protocol로 commit한다.
  - branch를 push하고 `Closes #77`이 포함된 PR을 만든다.
  - issue #77의 PR metadata를 맞춘다. assignee는 `debop`, milestone은 `0.6.0`, label은 `enhancement`와 `examples`다.
  - PR body는 `## DoD Status`로 끝낸다.
  - CI가 통과하면 merge하고, local `develop`을 sync하고, worktree 및 local/remote feature branch를 제거한다.

## Acceptance Criteria 매핑

| Spec 요구사항 | 계획 범위 |
|---|---|
| `examples/` 아래 실행 가능한 예제 | A, B, C, D |
| Currency-specific rounding | A, B, E |
| Discount eligibility | A, B, E |
| Invalid currency rejection | A, B, C, E |
| README의 #45 link | D |
| 검증 gate | E, F |
| Review와 PR metadata | G, H |

## 위험 가정

- `regional-vat`는 설명용이며 production tax compliance로 설명하면 안 된다.
- 설계상 conversion policy는 없다. 필요하면 downstream 예제가 settlement를 추가할 수 있다.
- Rule decision은 의도적으로 재사용 framework가 아니라 plain struct다.
