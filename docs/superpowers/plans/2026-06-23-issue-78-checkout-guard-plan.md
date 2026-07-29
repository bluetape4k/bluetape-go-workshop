# Issue #78 구현 계획

## 목표

`examples/checkout-guard-integration`를 테스트, 이중 언어 문서, 로컬 검증, PR, CI, merge, 로컬 동기화, worktree 정리까지 포함해 전달한다.

## 단계

1. #78과 focused example #44/#45/#46/#76/#77의 요구사항을 고정한다.
2. 인가된 checkout, claim 거부, 규칙 거부, 중복 제출, money 검증, 동시 admission, loopback 서버 설정에 대한 실패 서비스/HTTP 테스트를 추가한다.
3. `internal/checkoutguard`를 구현한다.
   - JWT demo token issue와 access-token verification.
   - Checkout request validation과 single-currency money pricing.
   - VIP discount, EU tax, restricted category에 대한 로컬 규칙 결정.
   - Idempotency key 기준 Bloom filter admission.
   - 공개 오류 allowlist와 Gin router.
4. loopback-only bind resolution과 제한된 HTTP timeout을 포함해 `main.go`를 추가한다.
5. 영어/한국어 example README, root README entry/run section, lesson note를 추가한다.
6. 대상 테스트, race test, 전체 test suite, lint/tidy/fmt/vet, diff hygiene를 검증한다.
7. 변경 파일을 code review하고, Lore trailer로 commit하고, #78과 metadata가 일치하는 PR을 만들고, CI를 기다린 뒤 merge하고 `develop`을 sync하고 feature worktree를 제거한다.

## Step DoD

| Step | 완료 조건 |
|---|---|
| 1 | Spec과 review가 #78 acceptance 및 non-goal을 포착한다. |
| 2 | `checkoutguard` 구현 누락 때문에 테스트가 실패한다. |
| 3 | 새 example package가 targeted test를 통과한다. |
| 4 | `main.go` 테스트가 통과하고 서비스를 로컬에서 smoke-run할 수 있다. |
| 5 | Example 문서와 root 문서가 bilingual이며 일관된다. |
| 6 | 필요한 로컬 검증 명령이 통과하거나 명시적으로 문서화된다. |
| 7 | PR이 merge되고, 로컬 `develop`이 `origin/develop`과 같으며 오래된 worktree가 남지 않는다. |
