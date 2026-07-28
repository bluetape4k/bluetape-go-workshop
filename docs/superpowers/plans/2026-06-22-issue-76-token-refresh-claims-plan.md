# Token Refresh와 Claims Validation 예제 구현 계획

> **에이전트 작업자 참고:** 작업 단위로 구현한다. 같은 branch에서 이 계획을 실행할 때 checkbox 상태를 최신으로 유지한다.

**목표:** `github.com/bluetape4k/bluetape-go/jwt`를 사용해 access-token claim을 검증하고 refresh token을 교환하는 focused Gin 예제 `examples/token-refresh-claims`를 추가한다.

**아키텍처:** 예제 내부 전용 `internal/tokenrefresh` 패키지 하나가 token issue, access validation, refresh validation, 공개 오류 매핑, Gin router를 소유한다. `main.go`는 loopback HTTP 서버 연결만 담당한다. 테스트는 결정적 clock, secret, ID generator로 계약을 고정한다.

**기술 스택:** Go, Gin, `github.com/bluetape4k/bluetape-go/jwt`, 표준 라이브러리 `errors`, `net/http`, `strings`, `time`. 새 직접 의존성은 추가하지 않는다.

## 제약

- `bluetape-go/jwt` helper를 직접 사용한다.
- 예제는 애플리케이션 형태로 유지한다. 재사용 가능한 auth/session 코드는 다른 곳에 둔다.
- 구현 전에 테스트를 먼저 작성한다.
- 공개 오류 응답은 allowlist 기반으로 유지하고 secret을 노출하지 않는다.
- 문서는 영어/한국어 쌍을 유지하고 root navigation을 갱신한다.
- 데이터베이스, Redis, OIDC, JWKS, cookie session, revocation list, 범용 auth middleware framework는 추가하지 않는다.

## 계획 파일

- `examples/token-refresh-claims/main.go`
- `examples/token-refresh-claims/main_test.go`
- `examples/token-refresh-claims/README.md`
- `examples/token-refresh-claims/README.ko.md`
- `examples/token-refresh-claims/internal/tokenrefresh/service.go`
- `examples/token-refresh-claims/internal/tokenrefresh/service_test.go`
- `README.md`
- `README.ko.md`
- `docs/lessons/2026-06-22-token-refresh-claims.md`
- `docs/review/2026-06-22-issue-76-token-refresh-claims-code-review.md`

## 구현 작업

- [x] **A. 서비스 TDD red 테스트 [복잡도: 중간]**
  - session issue, 유효한 access-token profile, 만료된 access token, malformed/wrong-key token, 잘못된 audience, 잘못된 token-use, 누락된 scope, 유효한 refresh exchange, access-as-refresh 거부, refresh-as-access 거부 테스트를 추가한다.
  - 공개 오류가 원본 token, demo secret, parser diagnostic을 포함하지 않는지 검증한다.
  - `go test -count=1 ./examples/token-refresh-claims/internal/tokenrefresh`를 실행하고 예상되는 compile/fail 출력을 TDD 증거로 남긴다.

- [x] **B. 서비스 구현 [복잡도: 중간]**
  - sentinel error, DTO, `Service`, `IssueSession`, `ValidateAccess`, `RefreshAccess`, claim parsing helper, bearer-token 추출, 안정적인 오류 매핑, `NewRouter`를 구현한다.
  - `jwt.NewFixedHMACProvider(jwt.HS256, secret, jwt.WithClock(...), jwt.WithKeyIDGenerator(...))`를 사용한다.
  - 테스트에는 결정적으로 주입한 ID generation을 사용하고, production에는 `session_id`와 `jti`를 위한 단순 entropy-backed generator를 사용한다.
  - 집중 package test를 실행한다.

- [x] **C. Main 진입점 TDD/구현 [복잡도: 낮음]**
  - 기본 loopback 주소, 유효한 loopback override, non-loopback 거부, server timeout 테스트를 추가한다.
  - 기본값 `127.0.0.1:8097`, 제한된 HTTP server timeout, signal handling, graceful shutdown을 포함해 `main.go`를 구현한다.
  - `go test -count=1 ./examples/token-refresh-claims/...`를 실행한다.

- [x] **D. 문서 [복잡도: 중간]**
  - 시나리오, endpoint 표, 실행 명령, curl 흐름, valid profile, refresh, invalid token-use, production hardening 경계를 담은 영어/한국어 예제 README를 추가한다.
  - root `README.md`와 `README.ko.md`의 예제 표 및 실행 섹션을 갱신한다.
  - #44를 기반 ID/JWT 경계 예제로 연결한다.
  - `docs/lessons/2026-06-22-token-refresh-claims.md`를 추가한다.

- [x] **E. 집중 검증 [복잡도: 중간]**
  - 다음을 실행한다.
    - `go test -count=1 ./examples/token-refresh-claims/...`
    - `go test -race -count=1 ./examples/token-refresh-claims/...`
    - `go run ./examples/token-refresh-claims`로 live smoke를 실행해 `/healthz`, `/sessions`, `/profile`, `/tokens/refresh`를 확인한다.

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
  - `docs/review/2026-06-22-issue-76-token-refresh-claims-code-review.md`에 저장한다.
  - 모든 P0/P1 finding을 수정하고 영향받은 테스트를 다시 실행한다.

- [ ] **H. Commit과 PR [복잡도: 낮음]**
  - Lore protocol로 commit한다.
  - branch를 push하고 `Closes #76`이 포함된 PR을 만든다.
  - issue #76의 PR metadata를 맞춘다. assignee는 `debop`, milestone은 `0.6.0`, label은 `enhancement`와 `examples`다.
  - PR body는 `## DoD Status`로 끝낸다.

## Acceptance Criteria 매핑

| Spec 요구사항 | 계획 범위 |
|---|---|
| 실행 가능한 예제 | A, B, C, D |
| 유효한 claim validation | A, B, E |
| 만료 token 경로 | A, B, E |
| invalid claim 경로 | A, B, E |
| refresh exchange 동작 | A, B, D, E |
| README의 #44 link | D |
| 검증 gate | E, F |
| Review와 PR metadata | G, H |

## 위험 가정

- 이 예제는 의도적으로 stateless이며 refresh-token reuse detection을 시연하지 않는다. README는 durable session/revocation storage를 production hardening 항목으로 명시해야 한다.
- 고정 HMAC secret은 결정적 데모 자료로 의도적으로 commit한다. README는 production system이 이를 복사하면 안 된다고 명시해야 한다.
- refresh token과 access token은 데모에서 같은 signing material을 공유하지만 audience와 `token_use` claim을 분리한다. Production system은 별도 key를 사용할 수 있다.
