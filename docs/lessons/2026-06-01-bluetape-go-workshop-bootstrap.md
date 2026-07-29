# bluetape-go-workshop Bootstrap

## 맥락

`bluetape-go`에는 실행 가능한 웹 애플리케이션 예제가 필요하지만, library 저장소를 애플리케이션 showcase로 바꾸면 안 된다.

## 결정

애플리케이션 예제 저장소로 `bluetape-go-workshop`을 만든다. Redis leader web service 하나에서 시작하고, 이후 예제는 안정화된 `bluetape-go` milestone에 맞춰 추가한다.

## 결과

저장소에는 다국어 README, 로컬 개발 명령, CI, Nightly Testcontainers 검증, 실행 가능한 `chi` 기반 HTTP leader 예제 하나가 준비됐다.

## 검증

- `actionlint .github/workflows/ci.yml .github/workflows/nightly-tests.yml`
- `golangci-lint config verify`
- `make ci`
- `git diff --check`

## 이후 Guard

workshop 예제는 얇게 유지한다. 애플리케이션 wiring은 이 저장소에 두되, 재사용 가능한 support code는 `bluetape-go`로 되돌려야 한다.

routing 또는 middleware가 필요한 예제에서는 기본 lightweight web framework로 `chi`를 사용하되, `net/http` handler 호환성은 유지한다.
