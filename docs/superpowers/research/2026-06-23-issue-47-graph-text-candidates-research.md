# Issue #47 리서치: Graph와 Text 워크숍 후보

## 범위

- 저장소: `bluetape4k/bluetape-go-workshop`
- 이슈: #47 `[v0.7.0] Research graph and text workshop candidates`
- 상위 track: #31 `[v0.7.0] Plan post-utility workshop tracks`
- 상위 roadmap epic: #27
- 마일스톤: `0.7.0`
- 작업 유형: Type E - Research / Maintenance

## 확인한 출처

- GitHub issue #47, 상위 #31, follow-up workshop issue:
  - Text: #34, #53, #54, #55, #67
  - Graph: #36, #50, #51, #52, #69
- upstream bluetape-go research:
  - `/Users/debop/work/bluetape4k/bluetape-go/docs/research/2026-06-01-milestone-0.10.0-text-research.md`
  - `/Users/debop/work/bluetape4k/bluetape-go/docs/research/2026-06-01-milestone-0.12.0-graph-research.md`
- Kotlin workshop example:
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/kotlin/text-processing/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/spring-data/elasticsearch/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/graph/abuser-detection/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/graph/recommendation/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/graph/social-network/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/graph/knowledge-graph/README.md`
- upstream Kotlin package reference:
  - `/Users/debop/work/bluetape4k/bluetape4k-text/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-text/text-search/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-text/tokenizer-japanese/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-text/lingua/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-graph/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-graph/graph/graph-core/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-graph/graph-io/core/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-graph/graph-io/csv/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-graph/graph-io/graphml/README.md`
- issue #47 / #31 history와 현재 issue body에 대한 context-mode search.

## 현재 근거

#47은 구현 전 candidate selection을 요구한다. 제안 scope에는 예제가 아직 0.8.0과
0.9.0을 target해야 한다고 되어 있지만, 현재 상위 track #31은 text를 0.10.0(#34),
graph를 0.12.0(#36)에 매핑한다. 이 리서치는 현재 parent mapping을 따르고, 오래된
0.8.0/0.9.0 문장은 stale issue text로 취급한다.

upstream bluetape-go text research는 deterministic search와 masking을 먼저 선호한다.
full Korean/Japanese tokenizer 채택은 dependency, dictionary, license, binary-size
review 뒤에 두도록 명시한다. upstream graph research는 넓은 backend abstraction보다
하나의 유용한 domain workflow와 portable import/export를 먼저 선호한다.

Kotlin text 예제는 abuse-word filtering, language detection, normalization을 가장
강한 workshop-shaped lesson으로 보여 준다. Kotlin text-search package는 Aho-Corasick
multi-keyword search, Unicode normalization, word-boundary behavior,
case-insensitive search, tokenization helper, Flow API, profanity masking, Korean
normalization을 강조한다. Lingua와 Japanese tokenizer package는 가치 있는 reference지만,
첫 Go 워크숍 예제 기준으로는 dependency가 무겁다.

Kotlin graph 워크숍에는 여러 domain example이 있다. `abuser-detection`은 user,
device, IP, payment token, suspicious cluster를 모델링한 뒤 risky user를 ranking한다.
`recommendation`은 collaborative filtering과 friend-of-friend recommendation을 위해
product, purchase, follow를 모델링한다. `social-network`는 recommendation shape와
겹친다. `knowledge-graph`는 유용하지만 첫 graph workshop pass에는 더 넓고 추상적이다.
graph package reference는 graph I/O, traversal, path, component, cycle,
import/export contract도 보여 주며, 이들은 generic service layer 뒤에 숨기지 말고
보이게 유지해야 한다.

## 채택한 Text 후보

### #53 Text Moderation Masking

첫 text workshop example로 채택한다.

이유:

- `bluetape4k-text/text-search` search와 masking behavior에 직접 매핑된다.
- Kotlin workshop `kotlin/text-processing` abuse-word filtering lesson에도 매핑된다.
- deterministic, local, dependency-free 상태를 유지할 수 있다.
- model 또는 network dependency 없이 overlapping pattern, mixed Korean/English input,
  replacement boundary, allowlist, visible unsupported case를 워크숍에서 다룰 수 있다.

구현 경계:

- HTTP wrapper보다 domain package를 먼저 만든다.
- masking policy를 명시적이고 testable하게 유지한다.
- 이 이슈에서는 production tokenizer dependency를 도입하지 않는다.

### #54 Gin Text Search Service

#53이 domain behavior를 제공한 뒤 채택한다.

이유:

- text track의 가장 작은 public API example이다.
- workshop service boundary를 통해 search/mask behavior를 노출하면서 handler를 얇게
  유지한다.
- full language coverage를 약속하지 않고 Unicode와 normalization caveat를 문서화할 수
  있다.

구현 경계:

- workshop issue가 public HTTP API example을 요구하므로 Gin을 사용한다.
- HTTP validation과 response projection을 search/masking logic과 분리한다.

### #55 Tokenizer / Language Detection Feasibility

dependency-heavy production tokenizer port가 아니라 feasibility와 contract example로
채택한다.

이유:

- Kotlin reference는 유용한 tokenizer, language detection, mixed language behavior를
  보여 준다.
- upstream Go research는 full tokenizer adoption을 dependency review 뒤에 둔다.
- 워크숍은 unknown, ambiguous, mixed-language input에 대한 명시적 handling을 여전히
  가르칠 수 있다.

구현 경계:

- deterministic heuristic, fixture-driven test, 명확한 limitation으로 시작한다.
- 외부 tokenizer 또는 language detector는 별도의 dependency decision 뒤에만 추가한다.

### #67 Gin Content Moderation Workflow Integration

#53, #54, #55가 자리 잡은 뒤 text track integration issue로 채택한다.

이유:

- focused example을 하나의 현실적인 content moderation flow로 조합한다.
- search, masking, token/language classification, unsupported language handling을
  하나의 API에서 보여 줄 수 있다.
- scenario-shaped이며 얇은 package API tour가 되는 것을 피한다.

구현 경계:

- README에서 #53, #54, #55로 다시 연결한다.
- moderation decision을 deterministic하고 local하게 유지한다.

## 보류하거나 거부한 Text 후보

- #47에서 Spring Data Elasticsearch port는 거부한다. source example은 search
  infrastructure evidence로 유용하지만 external-service이고 Spring-shaped인 반면,
  #34/#53/#54/#55/#67은 local bluetape-go text behavior를 먼저 target한다.
- full Korean/Japanese production tokenizer example은 보류한다. package value는
  실제로 있지만, workshop code가 이에 의존하기 전에 dependency, dictionary, license,
  binary-size risk에 대한 별도 결정이 필요하다.
- generic text framework scaffolding은 거부한다. 워크숍에는 abstraction layer가 아니라
  observable behavior가 있는 scenario-shaped example이 필요하다.

## 채택한 Graph 후보

### #50 Graph Abuse Cluster

첫 graph workshop domain example로 채택한다.

이유:

- Kotlin `graph/abuser-detection` example에 직접 매핑된다.
- user, device, IP, payment token, shared identifier라는 구체적인 domain이다.
- 넓은 graph backend abstraction 없이 traversal과 cluster reasoning을 연습한다.
- 민감한 identifier를 hashed 또는 opaque value로 표현할 수 있어 security boundary를
  자연스럽게 가르친다.

구현 경계:

- 작은 deterministic fixture와 expected cluster output을 사용한다.
- README와 test에서 identifier를 opaque하게 유지한다.
- upstream Go graph API가 stable backend 하나를 이미 쉽게 만들지 않는 한 multi-backend
  adapter를 피한다.

### #51 Graph Recommendation

두 번째 graph domain example로 채택한다.

이유:

- Kotlin `graph/recommendation`에 매핑되고 `graph/social-network`의 friend-of-friend
  idea를 부분적으로 재사용한다.
- ranking과 tie-breaking을 deterministic하게 테스트하기 쉽다.
- domain은 워크숍에 충분히 익숙하면서도 traversal과 neighborhood reasoning을 증명한다.

구현 경계:

- 작은 fixture로 user, product, purchase, follow를 모델링한다.
- recommendation scoring을 local하고 transparent하게 유지한다.
- ranking, tie-breaking, empty recommendation case를 테스트한다.

### #52 Graph Import / Export Fixtures

upstream Go graph I/O helper를 사용할 수 있게 된 뒤 채택한다.

이유:

- core record, CSV, GraphML에 대한 graph I/O reference에 매핑된다.
- 이후 graph example에 reusable fixture import/export behavior를 제공한다.
- 작고 reviewable하게 유지될 때만 가치가 있다.

구현 경계:

- 작은 CSV, NDJSON 또는 GraphML fixture를 사용한다.
- upstream API가 해당 policy를 노출한다면 duplicate와 missing-endpoint handling을
  증명한다.
- upstream package surface가 준비되지 않았다면 custom workshop-only graph I/O layer를
  만들지 않는다.

### #69 Graph Risk Intelligence Integration

#50, #51, #52 뒤 graph track integration issue로 채택한다.

이유:

- import/export, traversal, scoring, reporting을 현실적인 risk workflow 안에서
  조합한다.
- 모든 primitive를 다시 가르치는 대신 focused graph example로 다시 연결할 수 있다.
- scenario-shaped graph workshop이라는 milestone goal에 맞는다.

구현 경계:

- risk scoring을 deterministic하고 explainable하게 유지한다.
- focused example이 이미 증명한 graph package surface만 사용한다.

## 보류하거나 거부한 Graph 후보

- #47에서 넓은 backend abstraction example은 거부한다. upstream graph research는 Cypher,
  Gremlin 또는 backend-specific behavior를 leaky generic query API 뒤에 너무 일찍 숨기지
  말라고 경고한다.
- multi-backend Testcontainers matrix는 보류한다. upstream package confidence에는
  유용하지만 첫 Go workshop graph example에는 너무 무겁다.
- 직접 Ktor 또는 Spring graph service port는 거부한다. Go workshop은 이슈에 HTTP
  boundary가 필요할 때만 Gin 또는 plain HTTP를 사용해야 한다.
- standalone knowledge-graph example은 보류한다. 유용하지만 abuse-cluster와
  recommendation보다 넓고 즉시성이 낮다. #50/#51/#52가 graph package surface를 증명한
  뒤 다시 검토한다.
- bare traversal snippet은 거부한다. #36은 isolated API demonstration이 아니라 domain
  graph scenario를 요구한다.

## 권장 순서

1. text track의 나머지보다 #53을 먼저 구현한다.
2. #54를 text behavior 위의 public API wrapper로 구현한다.
3. #55로 tokenizer/language detection이 heuristic에 머물지, dependency를 채택할지
   결정한다.
4. #67을 text integration example로 구현한다.
5. graph track의 나머지보다 #50을 먼저 구현한다.
6. #51을 두 번째 graph domain example로 구현한다.
7. upstream graph I/O가 충분히 안정되면 #52를 구현한다.
8. #69를 graph integration example로 구현한다.

## #47 DoD 근거

- 구체적인 source path는 `확인한 출처`에 나열했다.
- 채택한 example은 이유와 follow-up issue link와 함께 나열했다.
  - Text: #53, #54, #55, #67
  - Graph: #50, #51, #52, #69
- 거부하거나 보류한 candidate는 이유와 함께 나열했다.
- #47의 stale 0.8.0/0.9.0 wording은 현재 parent #31 milestone mapping 기준으로
  해소했다.
