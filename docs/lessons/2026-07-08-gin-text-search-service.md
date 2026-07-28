# Gin Text Search Service Lesson

Issue #54는 exact matching example보다 한 단계 위, 이후 workflow integration보다 한 단계
아래에 있어야 한다. 유용한 teaching boundary는 명확하다. Gin은 HTTP parsing과 public error
shape를 소유하고, reusable service는 policy compilation, Unicode boundary matching, overlap
handling, masking을 소유한다.

endpoint는 scenario-shaped로 유지한다. response가 match, summary count, masked text, Unicode
caveat를 포함한다면 단일 `POST /text/search-mask` endpoint면 충분하다. demo를 관련 없는 search
endpoint와 mask endpoint로 나누면 handler가 shared domain contract보다 더 중요해 보인다.

test는 boundary 양쪽을 모두 증명해야 한다.

- HTTP test는 status code, JSON field name, error code, handler가 thin delegate로 남는지를
  assert한다.
- domain test는 Korean text, overlap handling, word-boundary behavior, exact original-span
  masking을 assert한다.

acceptance criteria가 persistence, queue, external service, Docker-backed protocol
compatibility를 포함하지 않으므로 이 issue에는 Testcontainers 실행이 필요하지 않다. 올바른
validation은 deterministic Go test, race test, example execution, rendered README diagram
inspection이다.

Diagram QA lesson: route label이 card title과 짧은 connector corridor를 공유하면 HTTP diagram은
읽기 어려워진다. 긴 route label은 card heading에서 떨어진 작은 pill에 두고, PNG를 렌더링한 뒤
PR 전에 검사한다.
