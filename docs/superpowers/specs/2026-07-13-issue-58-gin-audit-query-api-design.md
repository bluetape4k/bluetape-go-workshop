# Issue #58 Gin Audit Query API 설계

## Status

Issue #58에 대한 user-approved design이며 bluetape-go v0.18.0 기준 implementation 준비가
끝났다. 승인된 delivery boundary는 PR creation, green CI, rebase merge, local `develop`
synchronization, owned worktree cleanup을 포함한다.

## Goal

storage와 pagination rule을 Gin과 독립적으로 유지하면서 immutable order audit entry를 strict하고
bounded한 HTTP API로 query하는 실행 가능한 Gin service를 추가한다. 이 예제는 aggregate-scoped
history search, revision-based continuation, detail lookup, typed audit error, raw audit
payload 및 metadata 주위의 trust boundary를 가르친다.

## Context and Current Evidence

Issue #58은 milestone track #35에서 완료된 audit order history example (#56) 뒤에 오고 SQL
outbox example (#57, #68)보다 앞선다. 현재 stable dependency와 workshop module baseline은
bluetape-go v0.18.0이다.

released `audit.HistoryReader`는 persistence를 노출하지 않고 `Find`, `LoadHistory`, `Latest`,
snapshot read를 제공한다. `audit.Query`는 exact aggregate, aggregate type, inclusive revision 및
recorded-at range, newest-first ordering, limit을 지원한다. `audit.MemoryRepository`는
goroutine-safe이고 context를 확인하며 defensive copy를 반환하고 filtering과 ordering 이후 limit을
적용한다. metadata predicate나 event-ID lookup은 제공하지 않는다.

기존 `gin-content-moderation-workflow`는 이 repository의 현재 Gin boundary pattern을 제공한다.
strict bounded JSON, disabled proxy trust, stable public error, loopback-safe default, owned
server timeout, joined graceful shutdown이 그 pattern이다. 이 예제는 해당 boundary rule을 빌리되 더
작은 audit query contract를 소유한다.

## Chosen Approach

주입된 `audit.HistoryReader` 위에 internal `auditquery.Service`를 둔다. search는 하나의 canonical
aggregate를 요구하고 POST JSON body의 filter를 받는다. service는 request를 `audit.Query`로 변환하고
`limit + 1`을 요청하며, requested limit까지만 반환하고 첫 unseen revision을 next inclusive boundary로
노출한다. detail lookup은 exact aggregate와 revision을 사용해 같은 `Find` API를 사용한다.

실행 가능한 application은 deterministic order event로 `MemoryRepository`를 seed하고 Gin engine을 만든
뒤 read operation만 제공한다. Gin은 transport validation, request deadline, redacted public error,
safe diagnostic, body closure를 소유한다. `main`은 listener policy와 shutdown을 소유한다.

## Alternatives

### GET search with query parameters

단순 filter에는 익숙하지만 revision/time filter와 미래 extension은 URL을 길고 어색하게 만든다. 또한
structured search input을 JSON으로 제출한다는 승인된 결정과 충돌한다. 거부한다. compact detail
identity만 GET path로 남긴다.

### Global cross-aggregate search

`audit.Query`는 aggregate type 또는 모든 entry로 query할 수 있지만 revision은 aggregate 안에서만
정렬된다. 따라서 revision continuation token은 aggregate를 넘어서면 모호해지고 global query는
authorization 및 tenant isolation risk를 키운다. 이 workshop API에서는 거부한다.

### Application-owned metadata filtering

service가 entry를 가져와 `Event.Metadata`를 직접 filter할 수도 있다. repository가 먼저 `Limit`을
적용하므로 application이 unbounded result를 scan하지 않는 한 page가 incomplete하거나 incorrect해진다.
library 또는 durable store가 metadata-aware query contract를 제공할 때까지 거부한다.

## Package and Files

```text
examples/gin-audit-query-api/
  main.go
  main_test.go
  README.md
  README.ko.md
  internal/auditquery/
    model.go
    service.go
    service_test.go
    fixture.go
    fixture_test.go
    server.go
    server_test.go
```

root `README.md`와 `README.ko.md`는 example을 link한다. Type A spec, plan, review, lesson
artifact는 repository의 기존 docs path 아래에 둔다. module, dependency, workflow, container,
database, changelog, public bluetape-go API, diagram 변경은 필요하지 않다.

## Service API Contract

framework-independent package는 다음 teaching surface를 노출한다.

```go
type ServiceConfig struct {
    DefaultLimit int
    MaximumLimit int
}

type AggregateRequest struct {
    Type string `json:"type"`
    ID   string `json:"id"`
}

type SearchRequest struct {
    Aggregate      AggregateRequest `json:"aggregate"`
    FromRevision   audit.Revision   `json:"from_revision,omitempty"`
    ToRevision     audit.Revision   `json:"to_revision,omitempty"`
    FromRecordedAt time.Time        `json:"from_recorded_at,omitempty"`
    ToRecordedAt   time.Time        `json:"to_recorded_at,omitempty"`
    NewestFirst    bool             `json:"newest_first,omitempty"`
    Limit          int              `json:"limit,omitempty"`
}

type NextPage struct {
    FromRevision audit.Revision `json:"from_revision,omitempty"`
    ToRevision   audit.Revision `json:"to_revision,omitempty"`
}

type Page struct {
    Limit   int       `json:"limit"`
    HasMore bool      `json:"has_more"`
    Next    *NextPage `json:"next,omitempty"`
}

type SearchResponse struct {
    Entries []audit.Entry `json:"entries"`
    Page    Page          `json:"page"`
}

func NewService(audit.HistoryReader, ServiceConfig) (*Service, error)
func (s *Service) Search(context.Context, SearchRequest) (SearchResponse, error)
func (s *Service) Get(context.Context, AggregateRequest, audit.Revision) (audit.Entry, error)
```

`NewService`는 nil 및 typed-nil reader, non-positive default 또는 maximum, maximum보다 큰
default, 100을 초과하는 maximum을 거부한다. zero-value `Service`는 `ErrInvalidConfig`로 fail
closed한다. nil context는 audit package와 맞게 `context.Background`로 정규화한다.

stable sentinel은 `ErrInvalidConfig`, `ErrInvalidRequest`, `ErrEntryNotFound`이다. service
validation은 자체 sentinel을 wrap하고, 존재하는 경우 underlying `audit.ErrInvalidAggregateID`,
`audit.ErrInvalidRevision`, `audit.ErrInvalidQuery`를 보존한다. reader failure는 `%w`로 wrap되어
`errors.Is`와 `errors.As`로 계속 inspect할 수 있다.

aggregate type과 ID는 trim되며 `audit.NewAggregateID` 호출 전에
`[A-Za-z0-9][A-Za-z0-9._-]{0,127}`과 일치해야 한다. 이 path-safe policy는 decoding ambiguity를
피하고 caller-controlled key를 bounded로 유지한다. revision zero는 omitted search bound로만 valid하다.
detail lookup에는 valid nonzero `audit.Revision`이 필요하다.

## Pagination and Filtering Contract

search는 항상 exact aggregate를 `audit.Query`에 제공한다. revision 및 recorded-at range는 inclusive이고,
`newest_first`가 true가 아니면 repository append order를 사용한다. service는 `limit`이 zero일 때 설정된
default를 사용하고, negative limit 또는 maximum 초과 limit을 거부한다.

repository는 `effectiveLimit + 1`을 받는다. extra entry가 없으면 `has_more`는 false이고 `next`는
생략된다. extra entry가 있으면 반환하지 않고 그 revision을 다음 inclusive boundary로 사용한다.

- ascending search는 `next.from_revision`을 반환한다.
- newest-first search는 `next.to_revision`을 반환한다.

caller는 원래 JSON filter를 다시 제출하고 반환된 revision boundary만 바꾼다. 첫 unseen revision을
사용하면 recorded-at filter가 중간 revision을 생략하는 경우에도 duplicate와 gap을 피할 수 있다. empty
match는 nil이 아닌 빈 `entries` array를 가진 successful response다. search는 404를 반환하지 않는다.
detail lookup은 exact revision이 없을 때 `ErrEntryNotFound`를 반환한다.

## HTTP Contract

route는 다음과 같다.

```text
GET  /healthz
POST /audit/history/search
GET  /audit/aggregates/:type/:id/revisions/:revision
```

POST route는 optional UTF-8 charset을 가진 `application/json`, identity content encoding, valid
UTF-8, 32 KiB maximum body, 하나의 JSON object, duplicate key 없음, unknown field 없음, trailing JSON
value 없음을 요구한다. Gin은 automatic unescaping 또는 trailing-slash redirect 없이 raw escaped path를
사용한다. path value는 canonical identifier pattern을 만족해야 한다.

successful search response는 `SearchResponse`를 사용한다. successful detail response는
`{"entry": <audit.Entry>}`를 사용한다. audit entry는 의도적으로 JSON payload, event metadata,
author, change metadata, snapshot metadata를 유지한다. empty slice는 `null`이 아니라 `[]`로 encode한다.

public error는 다음 형태를 사용한다.

```json
{"error":{"code":"invalid_audit_query","message":"audit query is invalid"}}
```

malformed transport input과 service `ErrInvalidRequest`는 400에 mapping한다. typed audit
aggregate/query/revision validation은 `invalid_audit_query`와 함께 400에 mapping한다. service error가
두 종류를 모두 wrap할 수 있으므로 이 더 구체적인 mapping을 자체 sentinel보다 먼저 평가한다. missing
detail은 404에 mapping한다. adapter-owned deadline expiry는 408에 mapping한다. unknown reader/service
failure는 500에 mapping한다. raw error value, aggregate ID, payload, metadata, request body는 public
message 또는 log에 절대 포함하지 않는다.

adapter는 2초 service deadline을 적용하고, 모든 request body를 닫고, trusted proxy를 비활성화하며,
method-not-allowed handling을 활성화한다. log에는 route template, method, status, low-cardinality code,
elapsed time만 남긴다. caller cancellation은 내부적으로 보존되며 client disconnect 이후 unreliable
write를 유발하지 않는다.

## Fixture and Runtime Contract

`SeedRepository`는 두 order aggregate에 대해 deterministic하고 검증된 `audit.Entry` value를 append한다.
`order-1001`은 created, confirmed, packed, shipped event를 가진다. `order-2001`은 created 및
cancelled event를 가진다. ID, idempotency key, author, UTC timestamp, payload, change, safe workshop
metadata는 고정이다. constructor 또는 append가 하나라도 실패하면 seeding은 즉시 실패한다.

default address는 `127.0.0.1:8080`이다. non-loopback `HTTP_ADDR`는
`ALLOW_UNAUTHENTICATED_REMOTE=1`이 없으면 거부한다. `http.Server`는 header, read, write, idle,
maximum-header limit을 소유한다. signal-driven shutdown은 5초 bound를 가지며, listen goroutine을 join하고
graceful shutdown이 실패하면 `Close`를 강제한다. test에서는 public listener를 열지 않는다.

## Failure Modes

1. malformed, oversized, duplicate-key, unsupported-media, unknown-field JSON은 service 호출 전에
   거부된다.
2. invalid aggregate, revision/time range, direction boundary, limit은 typed audit error를 내부적으로
   보존하면서 stable 400을 반환한다.
3. missing detail revision은 404를 반환한다. empty search는 빈 array와 함께 200으로 남아 caller가 lookup
   absence와 empty page를 구분할 수 있다.
4. reader cancellation, deadline, failure는 retry 없이 전파한다. HTTP adapter는 자신의 timeout과 client
   cancellation을 구분하고 raw audit content를 절대 log하지 않는다.
5. 미래 metadata predicate는 limited reader 위에 조용히 추가할 수 없다. 그렇게 하면 page completeness를
   위반하므로 새로 승인된 query contract가 필요하다.
6. graceful shutdown 실패는 forced close를 trigger하고 listener를 join한다. server goroutine을 남기면 안 된다.

## Security and Operations Boundaries

예제에는 authentication, authorization, tenant policy, rate limiting, durable retention, encryption,
field-level redaction이 의도적으로 없다. loopback에 bind하므로 기본값에서만 안전하다. production
deployment는 aggregate identity와 반환되는 모든 payload/metadata field를 authorize해야 하며, tenant scope를
request JSON이 아니라 authenticated server context에서 파생해야 한다. 또한 retention/deletion policy를
적용하고 비싼 history read를 rate-limit해야 한다.

`/healthz`는 process availability만 증명한다. `MemoryRepository`는 restart 시 모든 data를 잃으며
readiness 또는 durability proof가 아니다. diagnostic은 low-cardinality이고 caller-controlled identifier를
의도적으로 생략한다. README는 이 예제를 event sourcing, recovery, global audit search, exactly-once delivery,
production-ready access control로 설명하면 안 된다. fixed fixture와 page limit은 이 demo response를 제한한다.
production repository도 per-entry payload/metadata와 total response byte budget을 강제해야 한다.

## Testing

focused test는 다음을 증명한다.

- constructor, typed-nil, zero-value, ID, revision/time, limit validation
- duplicate 또는 gap 없는 ascending 및 newest-first `limit + 1` pagination
- inclusive revision/time filter, empty page, detail success/not-found,
  `errors.Is`/`errors.As` reader failure 보존
- deterministic validated fixture entry
- strict media type/encoding/UTF-8/body-size/duplicate/unknown/trailing input, method 및
  path behavior, response shape, body closure, timeout, caller cancellation, redacted error,
  low-cardinality log
- loopback policy, server timeout default, graceful shutdown, forced close, listener failure,
  goroutine join
- exact response assertion과 함께 goroutine-safe memory repository 대상 `go test -race` 아래의
  concurrent search 및 detail read

validation order는 focused RED/GREEN package test, focused race test, loopback `go run` plus
curl smoke check, `git diff --check`, repository-wide `make ci`다.

## Compatibility, Migration, and Rollback

예제는 workshop file만 추가하고 기존 dependency를 사용한다. public library API를 변경하지 않으며 migration도
필요하지 않다. rollback은 example, root navigation entry, durable workflow artifact를 삭제하는 것이다. fixture는
disposable이며 state migration path가 없다.

## Acceptance Criteria and DoD

- search는 POST JSON을 사용하고 metadata filtering을 invent하지 않은 채 지원되는 v0.18.0 audit query
  surface에 정확히 mapping한다.
- framework-independent service test는 filter, 두 pagination direction, detail lookup, empty result,
  typed error, cancellation, concurrent read safety를 증명한다.
- Gin 및 server test는 strict bounded input, safe error/logging, body/resource ownership,
  request/server timeout, joined shutdown을 증명한다.
- English 및 Korean README file은 run/curl example, expected response shape, continuation instruction,
  trust-boundary warning을 포함한다. root navigation은 두 locale 모두에서 example을 link한다.
- spec, plan, implementation, review, lesson, PR, CI, rebase merge, local sync, owned-worktree
  cleanup이 P0=0/P1=0으로 완료된다.

## Specification Review Record

active subagent interface가 mandatory `agent_type` field를 노출하지 않아 required installed-role
dispatch를 수행할 수 없었다. 따라서 main session이 이 artifact를 대상으로 여섯 번의 isolated read를 수행하고
workflow fallback으로 결과를 통합했다.

| Lens | Result | Resolution |
|---|---|---|
| Performance | P0=0, P1=0, P2=1 | page를 최대 100개 plus one lookahead로 유지하고 production entry/response byte budget을 문서화했다. |
| Stability | P0=0, P1=0 | context ownership, inclusive continuation, body closure, timeout behavior, forced close, listener join을 명시했다. |
| Security | P0=0, P1=0 | aggregate scope, strict input, proxy policy, loopback default, safe log, authorization gap, remote opt-in을 명시했다. |
| Operator/Ops | P0=0, P1=0 | health semantic, in-memory durability, diagnostic, shutdown, rollback, retention, rate-limit boundary를 명시했다. |
| Developer/API | P0=0, P1=0 after repair | specific audit-error mapping precedence, typed-nil/zero-value behavior, exact first-unseen continuation contract를 명확히 했다. |
| User/caller | P0=0, P1=0 | POST JSON, empty-search versus detail-not-found, returned next boundary, unsupported metadata filter, misuse warning을 명시했다. |
| Main integration | P0=0, P1=0 | pre-implementation status를 수정했다. unresolved contradiction, scope expansion, dependency, repository hazard가 남지 않았다. |
