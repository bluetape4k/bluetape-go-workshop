# Text Moderation Masking Lesson

Issue #53은 첫 focused v0.8.0 text example이다. 예제는 local하고 deterministic하게 유지한다.
application code는 moderation policy, allowlist meaning, public decision contract를 소유하고,
`textsearch`는 exact phrase matching, Unicode boundary filtering, NFC normalization,
deterministic span을 소유한다.

README는 exact search와 tokenization의 차이를 명시해야 한다. 이 예제는 language-specific
token을 선택하거나 model을 호출하거나 moderation provider를 사용하지 않는다. configured phrase를
검색하고, Unicode word-boundary helper를 적용하고, allowlisted span을 뺀 뒤 accepted match
span만 masking한다.

이 lesson의 minimum test suite는 overlapping entry, Korean text, allowlist subtraction,
replacement boundary, context cancellation, preview stability를 다룬다. boundary test에는
`badge`, `badwolf` 같은 단어를 포함해 substring replacement가 의도한 contract가 아님을
reader가 볼 수 있게 한다.

Diagram QA lesson: rendered PNG font stack이 해당 glyph를 포함한다고 증명되지 않으면 SVG
asset에 한국어 text를 넣지 않는다. 첫 rendered sequence image는 한국어 text를 missing glyph
box로 보여줬다. 최종 diagram은 한국어 예시는 README prose에 두고 rendered asset에는 ASCII
label을 사용한다.

같은 v0.8.0 text track의 follow-up example은 tokenizer selection, language detection, HTTP
routing, 더 큰 content workflow를 추가할 수 있다. 하지만 base contract를 흐리면 안 된다.
exact `textsearch` matching은 deterministic helper이지 complete moderation 또는 security
boundary가 아니다.
