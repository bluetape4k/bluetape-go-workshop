# Issue #118 Japanese Search Preparation Lesson

## 범위와 결정

Issue #118은 작은 Japanese product catalog를 deterministic term search와 support-text
masking용으로 준비하는 application-shaped command를 추가한다. 예제는 released `bluetape-go`
v0.18.0 package를 조합하고 service를 `internal/catalogprep` 아래에 둔다. workshop
infrastructure나 reusable library API를 만들지 않는다.

승인된 design은 세 단계가 명시적이다. 하나의 reusable Japanese tokenizer로 product/query term을
준비하고, 하나의 compiled dictionary로 support text를 masking하며, query-local term을 immutable
prepared catalog와 match한다. 이 분리는 token source span, mask span, prepared-index position이
우발적인 shared coordinate system을 갖지 않게 한다.

## 검색 모드와 Lifecycle

Kagome Search mode는 compound Japanese text에 search-oriented heuristic segmentation을
추가하므로 적절한 teaching mode다. 그래도 이것은 tokenization일 뿐이다. POS selection,
base-form projection, normalization, deduplication, all-term matching, SKU ordering은 여전히
보이는 application policy로 남는다.

`NewService`는 Search-mode tokenizer와 blockword dictionary를 한 번 만들고, catalog를 한 번
준비한 뒤 세 가지를 재사용한다. IPA dictionary는 startup, binary-size, memory cost를 가지므로
construction은 `Search`에 속하지 않는다. 각 search는 matcher를 local로 compile해 call 사이에
mutable matcher state를 공유하지 않는다. 이 bounded CPU-only example을 위해 context,
goroutine, timer, I/O, shutdown contract를 새로 만들지 않는다.

## 표현과 Boundary Contract

searchable representation과 source-location representation은 의도적으로 다르다.

- comparison/index term은 NFC-normalized Kagome base form을 사용하고, base form이 없으면
  normalized surface text로 fallback한다.
- 모든 token span은 원본 title 또는 support text에 대한 field-local,
  start-inclusive/end-exclusive UTF-8 byte offset으로 남는다.
- span은 normalized text, masked text, index text, rune, display column을 설명하지 않는다.

Japanese support text는 whitespace word boundary를 안정적으로 드러내지 않으므로 masking은
`BoundaryNone`을 사용한다. 따라서 blockword match는 semantic moderation이나 security boundary가
아니라 explicit substring policy다. match와 겹치는 selected support token은 inspectable하게
남지만 non-indexable이 된다. search는 명시적으로 space-delimited된 prepared index 위에서
`BoundaryUnicodeWord`를 사용해 query term이 다른 prepared term 내부에서 match되지 않게 한다.

## 소유권과 JSON 안정성

owned result slice는 non-nil empty slice로 만든다. stable JSON에서는 이 차이가 중요하다.
no-match output은 `null`이 아니라 `[]`여야 한다. `Products`는 product slice와 token metadata
map을 recursively copy하고, 모든 search는 matched-term slice를 clone한다. caller가 반환값을
mutate해도 concurrent call이 보는 prepared catalog는 바뀌지 않는다.

masking exclusion repair는 반복 token-by-match scan을 single ordered sweep으로 바꿨다.
monotonic match cursor를 전진시키면 half-open byte-span overlap semantics를 보존하면서
exclusion pass가 `O(tokens + matches)`가 된다.

## 동시성과 문서 수리

첫 contention proof는 scheduler timing에 의존할 수 있었다. 수리된 test는 deterministic arrival
barrier를 사용한다. 모든 task가 한 번 announce하고, 마지막 arrival이 모든 task를 release하며,
stress harness는 정확히 18 completion과 실제 overlap(`MaxConcurrent >= 2`)을 증명한다. 이는
scheduler-specific maximum을 주장하지 않으면서 sleep-based timing assumption을 제거한다.

영어/한국어 README review도 literal structural translation이 아니라 자연스러운 prose를 요구했다.
두 locale은 audience에 맞는 idiomatic language를 쓰면서 같은 lifecycle, normalization,
boundary, output, non-goal fact를 설명한다. representative output은 별도 contract로
hand-maintain하지 않고 runnable JSON에서 복사한다.

## 발견한 누락과 Review 수리

- empty result slice는 처음에 `null` encoding 위험이 있었다. constructor와 test는 이제 owned
  non-nil empty를 요구한다.
- nested overlap scan은 의도한 linear bound를 흐렸다. ordered sweep과 boundary-focused test로
  교체했다.
- reuse stress test는 flakiness 없이 overlap을 증명하기 위해 deterministic contention barrier가
  필요했다.
- bilingual prose는 factual parity review 뒤 별도 naturalness review가 필요했다.
- 첫 final verification은 19개 `revive` documentation finding과 1개 `staticcheck` S1009 finding을
  드러냈다. implementation lane은 package/export comment를 추가하고 assertion을 단순화한 뒤
  새 commit에서 전체 verification sequence를 다시 실행했다.
- plan은 installed role dispatch가 unavailable하다고 잘못 설명했다. corrected contract는 spawn
  schema에 별도 `agent_type` field가 없을 때 child prompt에 installed native role을 주입한다.
  executor, verifier, code-reviewer, writer lane을 사용했다.

## 최신 검증 Evidence

behavioral implementation, public documentation, review repair, final proof는 2026-07-12에
`7f21b9bbbf89948bb6e919b7f93d032527b002ea`까지 검증됐다. 이후 ledger-refresh commit은 이
lesson과 plan checklist만 바꾸며 code, test, dependency, public README는 바꾸지 않는다. 해당
commit은 자기 자신을 이름으로 적을 수 없으므로 exact SHA의 authority는 `git log`다.

| Command or check | Result | Evidence |
|---|---|---|
| `make fmt-check` | PASS | exit 0, 0.04s |
| `make tidy-check` | PASS | exit 0, 0.11s |
| `make vet` | PASS | exit 0, 0.57s |
| `make lint` | PASS | exit 0, 1.31s, `0 issues.` |
| `go test -count=1 ./examples/japanese-search-preparation/...` | PASS | exit 0, 0.73s; package 0.553s |
| `go test -race -count=1 ./examples/japanese-search-preparation/...` | PASS | exit 0, 6.69s; package 6.489s |
| `make ci` | PASS | exit 0, 60.60s; repository normal and race suites |
| Two `go run` executions | PASS | 0.89s and 0.61s; both 20,889 bytes and SHA-256 `89ce7bc9731a154910dd776500e61014c6c755481ccb73710793602859df7dd6` |
| JSON parsing and marker audit | PASS | scenario/tokenizer, three products/searches, exact mask and SKUs, no-match `[]`, commands, notes, and all required slices non-null |
| Locale/link commands from the plan | PASS | both example READMEs, both root links, run/race commands, and `git diff --check` |
| Module and negative-scope audit | PASS | `go.mod`/`go.sum` unchanged from `origin/develop`; production scan found no HTTP, ranking, persistence, new dependency, goroutine/lifecycle, or rune-offset contract |

line-by-line audit는 spec acceptance row 7개, planned behavior category 9개, implementation
task 6개, Definition-of-Done row 6개를 현재 code, test, CLI JSON 또는 paired documentation에
mapping했다. 최종 performance, stability, security, operator, developer/API, user/caller,
integration review는 P0=0, P1=0으로 수렴했다.

## 향후 Guard와 Delivery Boundary

변경이 deterministic all-term matching을 가진 작은 immutable in-memory catalog에 머무르는 동안
current spec은 닫힌 상태로 유지한다. HTTP, ranking, persistence, catalog update, 새 dependency,
non-Japanese routing, rune/display offset, shared mutable matcher, public compatibility surface를
추가하기 전에는 spec을 다시 연다. tokenization mode, normalization, mask boundary, index
encoding, span ownership 변경도 새 fixture와 focused normal/race proof가 필요하다.

delivery는 `7f21b9b` 위 local ledger-refresh commit에서 멈춘다. PR creation, issue edit, merge,
push, branch deletion, local synchronization은 이 task 밖이다. exact final local commit의
authority는 `git log`다. final severity count는 P0=0, P1=0이며 알려진 unresolved delivery
blocker는 없다.
