# Catalog Refresh Resilience Workshop 예제

## 맥락

issue #13은 `bluetape-go` v0.1.1 workshop 예제가 필요했다. milestone은 대부분 품질 마무리였지만, tag line에는 첫 retry와 timeout resilience primitive가 포함됐다. 예제는 더 넓은 HTTP resilience service를 중복하지 않아야 했고, 더 풍부한 README diagram으로 scenario를 설명해야 했다.

## 결정

`examples/catalog-refresh-resilience`를 plain domain code로 추가한다. SKU refresh job은 catalog `Source` function을 주입받고, retry를 outer policy로, timeout을 inner policy로 둔 `resilience.Run`으로 감싼다. 이렇게 하면 예제를 작게 유지하면서도 retry attempt마다 timeout budget이 만들어진다는 점을 증명할 수 있다.

README diagram은 `docs/images/readme-diagrams/` 아래에 SVG/PNG pair로 저장하고, flow와 policy sequence diagram에는 Graphviz evidence를 둔다.

## 결과

예제는 다음을 다룬다.

- transient upstream failure 이후 retry success;
- `errors.Is(err, resilience.ErrTimeout)`으로 보이는 slow source timeout;
- timeout을 last cause로 보존하는 retry exhaustion;
- policy 실행 전 invalid input check.

## 검증

- `codegraph init && codegraph index`
- `code-review-graph build --repo .`
- `go test -count=1 ./examples/catalog-refresh-resilience/...`
- `git diff --check`
- 다음 diagram PNG render와 visual inspection:
  - `catalog-refresh-scenario-flow.png`
  - `catalog-refresh-policy-sequence.png`
  - `catalog-refresh-outcome-matrix.png`

## 이후 Guard

작은 Go workshop 예제는 code path를 scenario-first로 유지하되, README는 operational shape를 설명할 만큼 충분히 풍부해야 한다. README diagram을 추가할 때는 PNG embed와 matching SVG source를 함께 commit하고, node-and-connector diagram에는 Graphviz evidence를 남긴다.
