# Issue #52 bounded graph import/export 코드 리뷰

## 검토 범위

- `examples/graph-import-export/**`
- 루트 `README.md`, `README.ko.md` navigation과 실행 안내
- Issue [#52](https://github.com/bluetape4k/bluetape-go-workshop/issues/52)의
  v0.10.0 완료 조건

기준선은 `origin/develop`이며 브랜치는
`feat/issue-52-graph-import-export`다. 별도 reviewer lane을 사용하지 않고
구현 후 main session에서 read-only checklist review를 수행했다.

## 판정

P0=0 P1=0

현재 구현을 막는 결함은 발견되지 않았다.

| 영역 | P0 | P1 | 근거 |
|---|---:|---:|---|
| Security | 0 | 0 | 전체 입력·NDJSON line·record·record 수를 bounded로 제한하고, GraphML XML directive/extension과 unknown key를 거부한다. custom error는 untrusted format/record 식별자를 그대로 출력하지 않는다. |
| Ops/SRE | 0 | 0 | 외부 서비스, goroutine, retry, durable state를 추가하지 않았다. blocking reader의 cancellation 책임과 caller deadline/close 경계를 README에 적었다. |
| Go 품질 | 0 | 0 | `context.Context`를 모든 import/export 경계에 전달하고 `errors.Is` 가능한 sentinel과 기존 `graphio` 오류를 보존한다. canonical ordering과 scalar normalization은 별도 함수로 분리했다. |
| 테스트·침묵 실패 | 0 | 0 | 두 format round-trip, ordering, duplicate ID, missing endpoint, wrong direction/partner, non-scalar, GraphML unsupported construct, oversized input, cancellation, CLI/documentation parity를 검증한다. |
| 문서·추적성 | 0 | 0 | bilingual example README와 root navigation이 format, limit, unsupported scope, 명령, summary와 정규화 결과 발췌를 설명한다. |

## 주요 근거

- `internal/graphimport.Import`는 caller context와 전체 입력 상한을 먼저
  확인한 뒤 기존 `graphio`/`graphml` parser를 호출하고, partner/domain
  invariant와 directed `Account -> Device` subset을 검증한다.
- `internal/graphimport.Export`는 vertex와 edge를 ID 오름차순으로 정렬하고
  NDJSON 또는 bounded GraphML writer에 전달한다.
- `normalizeScalar`는 문자열·불리언을 보존하고 정수 표현을 `int64`로
  단일화하며, 안전 범위를 벗어난 정수·복합 값·null·non-finite number를
  거부한다.
- `RunDemo`는 동일 fixture를 `ndjson`와 `graphml`로 왕복한 뒤 JSON 기준
  데이터가 같은지 비교하고, `EncodeDemo`는 runtime duration 없이 안정적인
  report를 만든다.

## 검증 자료

- `go test -count=1 ./examples/graph-import-export/...`
- `go test -race -count=1 ./examples/graph-import-export/...`
- `go vet ./examples/graph-import-export/...`
- `go test -run '^Example' ./examples/graph-import-export/...`
- `make tidy-check fmt-check vet lint`
- `make test`
- `make race`
- `go run ./examples/graph-import-export`
- `node /Users/debop/.codex/skills/bluetape-writer/scripts/audit-korean-terms.mjs examples/graph-import-export/README.ko.md README.ko.md`
- `git diff --check`

모든 Go 검증 명령과 예제 실행은 성공했다. 한국어 용어 감사에서 새
`graph-import-export` 문서는 오류가 없었다. 루트 README의 기존 cache 예제에
있는 `snapshot` 용례 9건은 이번 변경이 도입하지 않은 context-specific
용례이므로 그대로 두고 이슈 범위 밖 예외로 기록한다.

## 잔여 경계

- CSV는 released `graph/graphio`의 paired API로 계속 제공되지만, 이 lesson의
  비교 CLI는 이슈가 지정한 기본 NDJSON와 named-partner GraphML 두 format만
  다룬다.
- Testcontainers, graph database, compression, filesystem ownership,
  atomic replacement, broad GraphML/yEd/yFiles compatibility는 이 이슈의
  비목표다.
- 취소 시 이미 block된 caller-owned reader의 `Read` 호출을 강제로 깨우는
  책임은 이 예제가 소유하지 않으며, caller가 close 또는 deadline을 제공해야
  한다.

## 최종 판정

P0=0, P1=0으로 Step 6-R review를 통과했다. PR과 merge 전에는 exact head,
CI, review/thread, issue/PR metadata를 다시 읽어야 한다.
