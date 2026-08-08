# Gin 텍스트 검색 서비스 교훈

Issue #54는 정확 일치 예제보다 한 단계 위에, 이후 workflow 통합보다 한 단계 아래에 둔다. 유용한
교육 경계는 명확하다. Gin은 HTTP 파싱과 공개 오류 형식을 담당하고, 재사용 가능한 서비스는 정책
컴파일, Unicode 경계 일치, 겹침 처리, 마스킹을 담당한다.

endpoint는 시나리오 중심으로 유지한다. response에 match, 요약 개수, 마스킹된 텍스트, Unicode
주의점이 포함된다면 단일 `POST /text/search-mask` endpoint면 충분하다. demo를 관련 없는 search
endpoint와 mask endpoint로 나누면 handler가 공유 도메인 계약보다 더 중요해 보인다.

테스트는 경계 양쪽을 모두 증명해야 한다.

- HTTP 테스트는 상태 코드, JSON 필드 이름, 오류 코드, handler가 얇은 위임으로 남는지를
  assert한다.
- 도메인 테스트는 한국어 텍스트, 겹침 처리, 단어 경계 동작, 원본 span의 정확한 마스킹을
  assert한다.

acceptance criteria가 persistence, queue, external service, Docker 기반 protocol compatibility를
포함하지 않으므로 이 issue에는 Testcontainers 실행이 필요하지 않다. 올바른 검증은 결정적인 Go
테스트, race 테스트, 예제 실행, 렌더링된 README 다이어그램 검사다.

다이어그램 QA 교훈: route label이 card title과 짧은 connector corridor를 공유하면 HTTP
다이어그램을 읽기 어려워진다. 긴 route label은 card heading에서 떨어진 작은 pill에 두고, PNG를
렌더링한 뒤 PR 전에 검사한다.
