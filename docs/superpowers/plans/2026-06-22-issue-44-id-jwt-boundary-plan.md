# ID와 JWT 경계 예제 구현 계획

> **에이전트 작업자 참고:** 필수 하위 스킬: `superpowers:subagent-driven-development`(권장) 또는 `superpowers:executing-plans`를 사용해 이 계획을 작업 단위로 구현한다. 단계 추적에는 체크박스(`- [ ]`) 문법을 사용한다.

**목표:** `examples/id-jwt-boundary`를 추가한다. 이 Gin 실행 예제는 `bluetape-go/id` UUID v7 식별자와 `bluetape-go/jwt` 고정 HMAC 데모 토큰을 함께 사용하면서, 식별자, 서명된 claim, 인가 정책의 경계를 설명한다.

**아키텍처:** 예제 내부 전용 `internal/idjwtboundary` 패키지 하나를 둔다. 서비스는 토큰 발급, 토큰 검증, 역할/scope 검사, 주문 요청 검증, UUID v7 생성, 안정적인 공개 오류 매핑을 소유한다. `main.go`는 loopback HTTP 서버 연결만 담당한다. 테스트는 결정적 clock/token/ID 의존성을 주입해 계약을 먼저 고정한다.

**기술 스택:** Go, Gin, `github.com/bluetape4k/bluetape-go/id`, `github.com/bluetape4k/bluetape-go/jwt`, 표준 라이브러리 `errors`, `net/http`, `strings`, `sync`, `time`. 새 직접 의존성은 추가하지 않는다.

---

## 제약

- `$bluetape-go-patterns`를 적용한다. 관련 경계에는 context-aware 처리를 사용하고, sentinel error, table-driven test, race-safe 의존성, `gofmt`를 지킨다.
- 예제는 애플리케이션 형태로 유지한다. 재사용 가능한 라이브러리 코드는 `bluetape-go`에 둔다.
- 구현 전에 테스트를 먼저 작성한다.
- 공개 오류 응답은 allowlist 기반으로 유지하고 secret을 노출하지 않는다.
- 문서는 영어/한국어 쌍을 유지하고 root navigation을 갱신한다.
- 데이터베이스, Redis, Testcontainers, session, OIDC, JWKS, 범용 auth middleware framework는 추가하지 않는다.

## 계획 파일

- `examples/id-jwt-boundary/main.go`
- `examples/id-jwt-boundary/main_test.go`
- `examples/id-jwt-boundary/README.md`
- `examples/id-jwt-boundary/README.ko.md`
- `examples/id-jwt-boundary/internal/idjwtboundary/service.go`
- `examples/id-jwt-boundary/internal/idjwtboundary/service_test.go`
- `README.md`
- `README.ko.md`
- `docs/lessons/2026-06-22-id-jwt-boundary.md`
- `docs/review/2026-06-22-issue-44-id-jwt-boundary-code-review.md`

## 구현 작업

- [ ] **A. 서비스 TDD red 테스트 [복잡도: 중간]**
  - 유효한 토큰의 주문 생성, 누락된 토큰, 만료된 토큰, 잘못된 형식의 토큰, 잘못된 키로 서명된 토큰, 금지된 scope, 잘못된 JSON/요청 검증, UUID v7 형태, ID generator 실패 테스트를 만든다.
  - 공개 오류가 원본 토큰, 데모 secret, 원본 JWT parser 문구를 포함하지 않는지 검증한다.
  - `go test -count=1 ./examples/id-jwt-boundary/internal/idjwtboundary`를 실행하고 예상되는 compile/fail 출력을 TDD 증거로 남긴다.

- [ ] **B. 서비스 구현 [복잡도: 중간]**
  - sentinel error, DTO, `Service`, `IssueToken`, `CreateOrder`, claim parsing helper, bearer-token 추출, 안정적인 오류 매핑, `NewRouter`를 구현한다.
  - `jwt.NewFixedHMACProvider(jwt.HS256, secret, jwt.WithClock(...), jwt.WithKeyIDGenerator(...))`를 사용한다.
  - production wiring에서는 `id.NewUUIDV7Generator`를 사용하고, 테스트에서는 generator를 주입한다.
  - 집중 package test를 실행한다.

- [ ] **C. Main 진입점 TDD/구현 [복잡도: 낮음]**
  - 기본 loopback 주소, `HTTP_ADDR` override, non-loopback 거부, server timeout 테스트를 추가한다.
  - 기본값 `127.0.0.1:8096`, 제한된 HTTP server timeout, signal handling, graceful shutdown을 포함해 `main.go`를 구현한다.
  - `go test -count=1 ./examples/id-jwt-boundary/...`를 실행한다.

- [ ] **D. 문서 [복잡도: 중간]**
  - 시나리오, endpoint 표, 실행 명령, curl 흐름, valid/missing/expired/invalid/forbidden 예시, production hardening 경계를 담은 영어/한국어 예제 README를 추가한다.
  - root `README.md`와 `README.ko.md`의 예제 표 및 실행 섹션을 갱신한다.
  - `docs/lessons/2026-06-22-id-jwt-boundary.md`를 추가한다.

- [ ] **E. 집중 검증 [복잡도: 중간]**
  - 다음을 실행한다.
    - `go test -count=1 ./examples/id-jwt-boundary/...`
    - `go test -race -count=1 ./examples/id-jwt-boundary/...`
    - `go run ./examples/id-jwt-boundary`로 live smoke를 실행해 `/healthz`, token issue, valid order, missing token, expired token, malformed token, forbidden scope를 확인한다.

- [ ] **F. 전체 repository 검증 [복잡도: 높음]**
  - 다음을 실행한다.
    - `go test -p 1 ./...`
    - `make fmt-check`
    - `make tidy-check`
    - `make vet`
    - `make lint`
    - `GOFLAGS=-p=1 make ci`
    - `git diff --check`
  - 범위 안의 실패를 수정한다.

- [ ] **G. Step 6-R code review와 수정 [복잡도: 중간]**
  - six-lane review와 security/trust-boundary review를 실행한다.
  - `docs/review/2026-06-22-issue-44-id-jwt-boundary-code-review.md`에 저장한다.
  - 모든 P0/P1 finding을 수정하고 영향받은 테스트를 다시 실행한다.

- [ ] **H. Commit과 PR [복잡도: 낮음]**
  - Lore protocol로 commit한다.
  - branch를 push하고 `Closes #44`가 포함된 PR을 만든다.
  - issue #44의 PR metadata를 맞춘다. assignee는 `debop`, milestone은 `0.6.0`, label은 `enhancement`와 `examples`다.
  - PR body는 `## DoD Status`로 끝낸다.

## Acceptance Criteria 매핑

| Spec 요구사항 | 계획 범위 |
|---|---|
| 내부 주문 ID와 요청 토큰 흐름 | A, B, D, E |
| JWT 발급/검증과 invalid 거부 | A, B, D, E |
| 만료 토큰 경로 | A, B, D, E |
| ID 형태 테스트 | A, B |
| Secret handling과 trust-boundary 문서 | D, G |
| Root navigation | D |
| 검증 gate | E, F |
| Review와 PR metadata | G, H |

## 위험 가정

- `go mod tidy`는 workshop module에서 아직 직접 사용하지 않는 `bluetape-go/id`와 `bluetape-go/jwt` 전이 의존성의 checksum을 추가할 수 있다.
- 고정 HMAC secret은 결정적 데모 자료로 의도적으로 commit한다. README는 production system이 이를 복사하면 안 된다고 명시해야 한다.
- 이 예제는 완전한 인가 시스템이 아니다. 역할/scope 검사는 경계 학습을 위한 local policy hook일 뿐이다.
