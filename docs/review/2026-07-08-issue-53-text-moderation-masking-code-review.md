# Issue #53 텍스트 모더레이션 마스킹 코드 리뷰

범위: `examples/text-moderation-masking`, 루트 README 카탈로그 항목, README
다이어그램 자산, #53 lesson/review 산출물.

기준선: `origin/develop` 대비 로컬 브랜치
`feat/issue-53-text-moderation-masking`.

## 발견 사항

P0=0 P1=0

남아 있는 차단 이슈는 없다.

## 검증 자료

- `DefaultPolicy`는 겹치는 영어 차단어, 한국어 차단어, 허용 문구,
  Unicode 경계 매칭, NFC 정규화를 정의한다:
  `examples/text-moderation-masking/internal/moderation/service.go:78`.
- `Evaluate`는 요청을 검증하고 호출자 취소를 반환하며, 허용 span을
  찾고 포함된 차단어 매치를 걸러낸 뒤 정확한 span만 마스킹한다. 또한
  발견 항목과 허용 목록 hit를 분리해서 반환한다:
  `examples/text-moderation-masking/internal/moderation/service.go:122`.
- 테스트는 한국어 텍스트가 포함된 겹침 패턴, 허용 목록 차감, 치환 경계,
  취소 전파, preview 안정성을 다룬다:
  `examples/text-moderation-masking/internal/moderation/service_test.go:11`.
- README 파일은 exact search와 tokenization의 차이, architecture/sequence
  다이어그램, 실행/테스트 명령, 네트워크 및 모델 경계를 설명한다:
  `examples/text-moderation-masking/README.md:8`.
- 루트 README 파일은 새 v0.8.0 예제를 카탈로그와 실행/테스트 명령
  섹션에 포함한다: `README.md:57`.

## 검증

- `go test -count=1 ./examples/text-moderation-masking/...`
- `go test -race -count=1 ./examples/text-moderation-masking/...`
- `go run ./examples/text-moderation-masking`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `make ci`
- `xmllint --noout docs/images/readme-diagrams/text-moderation-masking-architecture.svg docs/images/readme-diagrams/text-moderation-masking-sequence.svg`
- `/Users/debop/.local/bin/cairosvg ... -s 2` for both SVG diagrams
- Full-size PNG inspection for both diagrams
- `git diff --check`

## 검증 공백

이 이슈에는 Testcontainers 실행이 필요하지 않다. 인수 조건은 네트워크나
모델 의존성이 없는 로컬 텍스트 예제를 요구하므로, 관련 검증은
결정적인 Go 테스트, race 테스트, 예제 실행, 렌더링된 README 다이어그램
점검으로 충분하다.
