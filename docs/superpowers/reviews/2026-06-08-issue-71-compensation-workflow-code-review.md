# Step 6-R 코드 리뷰: Issue #71 Compensation Workflow

## 범위

- Branch: `feat/issue-71-compensation-workflow`
- Baseline: `origin/develop`
- 검토 파일:
  - `examples/compensation-workflow/**`
  - `scripts/generate-compensation-workflow-diagrams.sh`
  - `docs/images/readme-diagrams/compensation-workflow-*`
  - `README.md`, `README.ko.md`
  - `docs/images/readme-diagrams/workshop-example-map.*`
  - `docs/lessons/2026-06-08-compensation-workflow.md`

## 리뷰 반복

| 반복 | finding | 해결 |
|---|---|---|
| 1 | P1 candidate: reversible forward step 뒤 caller cancellation이 발생했을 때 compensation이 cancelled request context를 사용하면 cleanup이 실행되지 않을 수 있었다. | side effect 뒤 forward report가 cancelled이면 registered compensation을 `context.WithoutCancel(ctx)`로 실행하도록 수정하고 `TestCompensationRunCancellationAfterSideEffectStillCleansUp`을 추가했다. |
| 2 | diagram review finding: final README diagram이 raw Graphviz evidence처럼 보였고 workshop baseline decorator frame, visual band, footer, 더 풍부한 route context가 빠져 있었다. | Graphviz `.dot`, `.plain`, `*-graphviz.*` file은 route evidence로 유지하면서 final scenario, architecture, sequence SVG/PNG asset을 decorated hand-authored README diagram으로 재작업했다. 세 PNG를 다시 render하고 시각적으로 검사했다. |
| 3 | diagram review finding: frame Top/Bottom/Left/Right margin이 특히 sequence diagram에서 시각적으로 불균형했다. | sequence participant/lifeline/message body를 중앙 정렬하고 footer를 frame에 맞게 넓혔으며, L/R/T/B value에 대한 explicit generator margin output과 failure gating을 추가했다. |

## 7-Tier finding

| Tier | 결과 | 근거 |
|---|---|---|
| 1. Security | P0=0 P1=0 | auth/trust boundary나 unsafe deserialization은 추가되지 않았다. input은 JSON binding과 `server.go`의 blank `order_id` validation으로 제한된다. |
| 2. Ops/SRE reliability | P0=0 P1=0 | `main.go`는 explicit `ReadHeaderTimeout`을 사용한다. `/healthz`가 있으며 status mapping은 success, conflict, cancellation, bad request를 다룬다. |
| 3. Structural impact | P0=0 P1=0 | 새 example package만 추가되며 reusable `bluetape-go` API나 shared package는 변경되지 않았다. |
| 4. Go code quality | P0=0 P1=0 | `context.Context`는 workflow step을 통해 전파된다. compensation은 original error를 보존한다. revive return-order issue 수정 뒤 `make ci` lint가 통과했다. |
| 5. Tests/types/silent failure | P0=0 P1=0 | test는 success, shipment failure, payment failure, inventory failure, compensation failure, bad request, caller cancellation, side effect 뒤 cancellation, parallel request state isolation을 다룬다. |
| 6. Performance/stability | P0=0 P1=0 | goroutine, timer, retry loop, external IO가 추가되지 않았다. request-scoped mutable state는 run별로 격리되고 race-tested 상태다. |
| 7. Docs/release/evidence | P0=0 P1=0 | EN/KO README는 scenario, architecture, sequence diagram, run instruction, production durability caveat을 포함한다. diagram PNG/SVG/DOT/PLAIN artifact가 있고 시각적으로 검사했다. |

## 검증 근거

```bash
./scripts/generate-compensation-workflow-diagrams.sh
git diff --check
go test -count=1 ./examples/compensation-workflow/...
go test -race -count=1 ./examples/compensation-workflow/...
make ci
```

추가 diagram check:

```bash
rg -n ">[0-9]+<|>[0-9]+\\.<|undefined|Actor [0-9]|source to target" docs/images/readme-diagrams/compensation-workflow-*.svg || true
rg -n "Inter|Arial|Helvetica" docs/images/readme-diagrams/compensation-workflow-*.svg || true
rg -n "!\\[.*\\]\\(([^)]*\\.svg|[^)]*-graphviz|[^)]*\\.dot|[^)]*\\.plain)\\)" README.md README.ko.md examples/compensation-workflow/README.md examples/compensation-workflow/README.ko.md || true
```

## Gate 판정

P0=0 P1=0. Step 6-R이 통과했다.
