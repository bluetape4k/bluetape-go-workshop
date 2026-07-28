# Issue #46 설계: Probabilistic Admission and Dedupe 예제

## 목표

`github.com/bluetape4k/bluetape-go/probabilistic`으로 event admission과 duplicate
prefiltering을 보여주는 실행 가능한 `examples/probabilistic-dedupe-admission`
Gin API를 추가한다.

이 예제는 definitely-new event와 probably-seen event의 차이를 보여주고, 테스트와
문서를 위해 입력을 결정적으로 유지해야 한다. 또한 Bloom filter는 false positive를
반환할 수 있으며 production workflow에서는 durable authoritative storage와 함께
사용해야 한다는 점을 설명해야 한다.

## 현재 근거

- Issue #46은 definitely-new path와 probably-seen path 테스트를 포함한
  scenario-shaped probabilistic admission 또는 dedupe 예제를 요구한다.
- `bluetape-go` v0.6.2는 `github.com/bluetape4k/bluetape-go/probabilistic`을
  노출한다.
- `probabilistic.NewStringBloomFilter`는 `MightContain`, `Put`,
  `ApproximateElementCount`, `ExpectedFPP`를 가진 in-memory goroutine-safe Bloom
  filter를 제공한다.
- 현재 workshop에는 probabilistic 예제가 아직 없다.
- 최근 public HTTP 예제는 Gin, `/healthz`, loopback-only default binding, bounded
  JSON request body를 사용한다.

## 사용자 이야기

워크숍 독자로서 나는 webhook event ID를 작은 HTTP API에 제출하고, event가
definitely new로 admit되는지 probabilistic prefilter에서 probably seen으로
취급되는지 확인할 수 있다. 또한 approximate filter stat을 보고 production
service에 authoritative store가 여전히 필요한 시점을 이해할 수 있다.

## 범위

- `examples/probabilistic-dedupe-admission`을 추가한다.
- Internal package `examples/probabilistic-dedupe-admission/internal/dedupe`를
  추가한다.
- English/Korean example README를 추가한다.
- Root English/Korean README navigation을 갱신한다.
- `docs/lessons` 아래에 짧은 lesson을 추가한다.
- Full-feature spec, plan, review artifact를 추가한다.

## 비목표

- Durable storage를 추가하지 않는다. README는 production system에 durable storage가
  여전히 필요하다고 설명해야 한다.
- Redis-backed Bloom filter 또는 distributed dedupe를 구현하지 않는다. 현재
  package 문서는 이를 later work로 둔다.
- Bloom filter가 duplicate를 증명할 수 있다고 주장하지 않는다. Bloom filter는 ID가
  definitely new인지 probably seen인지만 말할 수 있다.
- 워크숍에 reusable dedupe framework를 추가하지 않는다.

## API 계약

### Endpoint

| Method | Path | 목적 |
|---|---|---|
| `GET` | `/healthz` | Process liveness 전용. |
| `POST` | `/events/admit` | Bloom prefilter를 통해 event ID 하나를 admit한다. |
| `GET` | `/filters/current` | Approximate filter statistic을 반환한다. |

### 요청

```json
{
  "event_id": "evt-1001",
  "source": "checkout"
}
```

`source`는 시나리오 현실감을 위해 받고 echo하지만, prefilter key는 trim된
`event_id`다.

### 응답

```json
{
  "event_id": "evt-1001",
  "source": "checkout",
  "decision": "admit",
  "reason": "definitely_new",
  "accepted": true,
  "stats": {
    "expected_insertions": 1000,
    "target_false_positive_probability": 0.01,
    "approximate_element_count": 1,
    "expected_false_positive_probability": 0.000000000000000006
  }
}
```

Decision 값:

- `admit`: insertion 전에 event가 definitely new였고 authoritative workflow로
  진행할 수 있다.
- `probably_seen`: event가 전에 처리되었을 수도 있고 false positive일 수도 있다.
  Demo는 prefilter boundary에서 이를 reject하지만, production에서는 중요한 데이터를
  drop하기 전에 durable storage를 확인해야 한다고 문서화한다.

### Public Error

| Code | HTTP | 의미 |
|---|---:|---|
| `invalid_request` | 400 | 잘못된 JSON 또는 누락/유효하지 않은 event field. |
| `filter_error` | 500 | 예상하지 못한 filter setup 또는 runtime error. |

## Acceptance Criteria

- `go test -count=1 ./examples/probabilistic-dedupe-admission/...`가 통과한다.
- `go test -race -count=1 ./examples/probabilistic-dedupe-admission/...`가
  통과한다.
- `go test -p 1 ./...`가 통과한다.
- Merge 전에 `make ci`가 통과한다.
- 테스트는 다음을 다룬다.
  - 첫 event가 `admit` / `definitely_new`인지
  - 반복 event가 `probably_seen`인지
  - invalid request mapping
  - assertion에 충분히 deterministic한 stat
  - HTTP success 및 duplicate path
- README.md와 README.ko.md는 false positive와 durable authoritative storage와의
  production pairing을 설명한다.
- Root English/Korean README는 새 예제를 link하고 `probabilistic`을 나열한다.

## Step 2-R Integrated Review

이 Codex surface에서는 native subagent spawning을 사용할 수 없으므로, 여섯 review
lane은 full-feature reference contract를 사용한 독립 main-session check로 실행했다.

| Priority | Area | Finding | Required spec edit |
|---|---|---|---|
| P2 | User | Bloom filter는 `probably_seen`만 보고할 수 있으며, wording이 duplicate proof를 암시하면 안 된다. | 명시적인 decision vocabulary와 non-goal을 추가했다. 완료. |
| P2 | Operator | Durable storage와의 production pairing은 핵심 issue acceptance criterion이다. | README 및 boundary requirement를 추가했다. 완료. |
| P2 | Stability | Shared filter state에는 race validation이 필요하다. | Changed-package race gate를 추가했다. 완료. |
| P3 | Developer | `source`가 dedupe key 의미를 복잡하게 만들면 안 된다. | Event ID가 prefilter key라고 명시했다. 완료. |

최종 Step 2-R 판정: P0 = 0, P1 = 0.
