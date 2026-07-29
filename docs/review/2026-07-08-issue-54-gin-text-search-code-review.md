# Issue #54 Gin 텍스트 검색 서비스 코드 리뷰

범위: `examples/gin-text-search-service`, 루트 README 카탈로그 항목, README
다이어그램 자산, #54 lesson/review 산출물.

기준선: `origin/develop` 대비 로컬 브랜치
`feat/issue-54-gin-text-search`.

## 발견 사항

P0=0 P1=0

남아 있는 차단 이슈는 없다.

## 검증 자료

- `NewServer`는 `/healthz`와 `POST /text/search-mask`만 연결한다.
  handler는 JSON을 bind하고 오류를 매핑한 뒤 검색 동작을
  `Service.SearchMask`에 위임한다:
  `examples/gin-text-search-service/internal/searchapi/service.go:183`.
- `Service.SearchMask`는 요청 형태를 검증하고 1-rune mask를 강제하며,
  컴파일된 `textsearch.Matcher`로 검색하고 원문 span만 정확히 마스킹한다.
  또한 요약 count와 Unicode caveat를 반환한다:
  `examples/gin-text-search-service/internal/searchapi/service.go:137`.
- 도메인 테스트는 한국어 텍스트, leftmost-longest 겹침, Unicode 경계 동작,
  사용자 지정 mask 출력, byte-span 증거를 다룬다:
  `examples/gin-text-search-service/internal/searchapi/service_test.go:13`.
- HTTP 테스트는 성공 응답 형태와 안정적인 검증 오류를 다룬다:
  `examples/gin-text-search-service/internal/searchapi/service_test.go:43`.
- README 파일은 curl 예제, endpoint contract, Unicode caveat,
  architecture/sequence 다이어그램, 집중 테스트 명령을 포함한다:
  `examples/gin-text-search-service/README.md:8`.

## 검증

- `go test -count=1 ./examples/gin-text-search-service/...`
- `go test -race -count=1 ./examples/gin-text-search-service/...`
- `go run ./examples/gin-text-search-service`
- Temporary loopback server check with `SERVE_HTTP=1 HTTP_ADDR=127.0.0.1:18098 go run ./examples/gin-text-search-service`, verifying HTTP 200 success JSON and HTTP 400 `invalid_request`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `make ci`
- `xmllint --noout docs/images/readme-diagrams/gin-text-search-service-architecture.svg docs/images/readme-diagrams/gin-text-search-service-sequence.svg`
- `/Users/debop/.local/bin/cairosvg ... -s 2` for both SVG diagrams
- Full-size PNG inspection for both diagrams
- `git diff --check`

## 검증 공백

이 이슈에는 Testcontainers 실행이 필요하지 않다. 예제는 데이터베이스,
queue, 모델, 외부 서비스 의존성이 없다. HTTP 동작은 `httptest`로
검증되고 도메인 동작은 결정적인 단위 테스트로 검증된다.

## 병합 후 다이어그램 재감사

범위: `docs/images/readme-diagrams/gin-text-search-service-architecture.*`
및 `docs/images/readme-diagrams/gin-text-search-service-sequence.*`.

수정된 발견 사항:

- SVG marker shape는 이제 고정 `userSpaceOnUse` filled-triangle path와
  `stroke="none"`, `stroke-dasharray="none"`을 사용한다. 그래서 CairoSVG
  PNG 출력에서 arrowhead 방향과 형태가 안정적으로 유지된다.
- Sequence message label은 call/return line에 닿지 않고 10px 위에 놓인다.
- Sequence participant, header, label, message class는 로컬 sequence-style
  audit contract와 일치한다.

검증:

- `xmllint --noout docs/images/readme-diagrams/gin-text-search-service-architecture.svg docs/images/readme-diagrams/gin-text-search-service-sequence.svg`
- `cairosvg docs/images/readme-diagrams/gin-text-search-service-architecture.svg -o docs/images/readme-diagrams/gin-text-search-service-architecture.png`
- `cairosvg docs/images/readme-diagrams/gin-text-search-service-sequence.svg -o docs/images/readme-diagrams/gin-text-search-service-sequence.png`
- `diagram-connector-audit.py`: PASS for both SVGs
- `diagram-geometry-audit.py --fail-diagonal`: `geometry_failures=0` for both SVGs
- `diagram-endpoint-audit.py`: PASS for both SVGs
- `diagram-mixed-corner-audit.py`: PASS for both SVGs
- `diagram-sequence-style-audit.py`: PASS for the sequence SVG
- Custom label-gap check: `labels=8 min_gap=10.0px failures=0`
- Full-size PNG inspection for both regenerated PNG files
- `git diff --check`

### 후속 Boundary 화살표 수정

PR #158 audit는 `Boundary -> Original byte spans` connector의 렌더링 방향성과
가독성 문제를 여전히 놓쳤다. 원인은 이전 route가 `Original byte spans`
card를 `Boundary` centerline보다 5px 왼쪽에서 끝내면서 terminal curve가
있는 dogleg를 강제한 것이다. CairoSVG는 기술적으로 유효한 marker를
렌더링했지만, PNG에서는 깔끔한 하향 관계로 읽히지 않았다.

수정:

- `Boundary` centerline이 대상 card의 top edge 안에 들어오도록
  `Original byte spans`를 335px에서 365px로 넓혔다.
- dogleg route를 단일 수직 helper connector로 대체했다:
  `M 1535 730 V 785`.
- 렌더링된 PNG crop과 full-size PNG에서 `Boundary`에서
  `Original byte spans`로 향하는 명확한 하향 arrowhead를 확인했다.

검증:

- `xmllint --noout docs/images/readme-diagrams/gin-text-search-service-architecture.svg`
- `cairosvg docs/images/readme-diagrams/gin-text-search-service-architecture.svg -o docs/images/readme-diagrams/gin-text-search-service-architecture.png -s 2`
- `diagram-connector-audit.py`: PASS
- `diagram-geometry-audit.py --fail-diagonal`: `geometry_failures=0`
- `diagram-endpoint-audit.py`: PASS
- `diagram-mixed-corner-audit.py`: PASS
- Custom invariant: `path=vertical_down`, `target_inside_top_guard=True`, `card_width=365`
- Full-size PNG inspection plus focused crop inspection for the boundary arrow
- `git diff --check`
