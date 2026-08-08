# 텍스트 모더레이션 마스킹 교훈

Issue #53은 첫 집중형 v0.8.0 text 예제다. 예제는 로컬에서 결정적으로 유지한다. 애플리케이션
코드는 모더레이션 정책, allowlist 의미, 공개 결정 계약을 담당하고, `textsearch`는 정확한 구문
일치, Unicode 경계 필터링, NFC 정규화, 결정적인 span을 담당한다.

README는 정확 검색과 tokenization의 차이를 명시해야 한다. 이 예제는 language-specific
token을 선택하거나 model을 호출하거나 moderation provider를 사용하지 않는다. 설정한 문구를
검색하고, Unicode word-boundary helper를 적용하고, allowlisted span을 뺀 뒤 승인된 match
span만 마스킹한다.

이 교훈의 최소 테스트 모음은 overlapping entry, 한국어 텍스트, allowlist subtraction,
replacement boundary, context cancellation, preview stability를 다룬다. 경계 테스트에는
`badge`, `badwolf` 같은 단어를 포함해 substring replacement가 의도한 계약이 아님을 독자가
확인할 수 있게 한다.

다이어그램 QA 교훈: 렌더링된 PNG 글꼴 스택이 해당 glyph를 포함한다고 증명되지 않으면 SVG
asset에 한국어 텍스트를 넣지 않는다. 첫 렌더링 sequence image는 한국어 텍스트를 missing glyph
box로 보여줬다. 최종 다이어그램은 한국어 예시를 README prose에 두고 렌더링된 asset에는 ASCII
label을 사용한다.

같은 v0.8.0 text 트랙의 후속 예제는 tokenizer selection, language detection, HTTP routing,
더 큰 content workflow를 추가할 수 있다. 하지만 기본 계약을 흐리면 안 된다. 정확한
`textsearch` matching은 결정적인 helper이지 완전한 moderation 또는 security boundary가 아니다.
