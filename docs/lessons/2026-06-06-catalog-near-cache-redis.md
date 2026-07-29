# Catalog Near-Cache Redis 예제 교훈

Issue: `#15 [v0.3.0] Add Redis near-cache and stampede coordination catalog example`

## 효과가 있었던 점

- 얇은 example-local `Peer` boundary만으로도 재사용 catalog infrastructure를 만들지 않고 `cache.NewMemory`, `redisnear.NewPubSub`, `rediscoord.NewStampedeCache`를 보여 줄 수 있었다.
- `Store.LoadCount`는 cache 동작을 관찰 가능하게 만들었다. peer invalidation reload와 cross-peer cold-miss coordination을 모두 증명했다.
- `bluetape-go/testing`을 example-local polling helper로 바꾸면서 issue의 no-new-dependencies acceptance criterion을 지켰다.
- Redis Testcontainers readiness에는 fixture startup 뒤 client `PING`이 필요했다. fixture log wait만으로는 race-test timing에서 항상 충분하지 않았다.

## Diagram 메모

- README diagram은 `README.md`와 `README.ko.md`에서 English label을 공유한다.
- 각 node-and-connector diagram은 final editable SVG와 rendered PNG pair를 README asset source로 유지한다.
- `Architects Daughter`와 `Comic Mono`는 renderer font fallback을 피하기 위해 final SVG asset에 `@font-face`로 명시적으로 load된다.
- visual inspection은 초기 scenario와 architecture render의 route/card overlap을 잡아냈다. layout review로 구조를 확정한 뒤 final SVG를 직접 patch했다.
- 이후 visual review는 split cold-miss와 invalidation scenario에서 피할 수 있는 connector crossing을 다시 잡았다. 먼저 request/response corridor를 분리하고, rendered arrowhead 앞에는 충분한 final straight segment를 남겨야 한다.

## 검증 메모

- `go test -count=1 ./examples/catalog-near-cache-redis/...`
- `go test -race -count=1 ./examples/catalog-near-cache-redis/...`
- `go test -count=10 -run TestColdMissBurstAcrossPeersRunsBackingLoaderOnce ./examples/catalog-near-cache-redis/...`
- `go test ./...`
- `golangci-lint cache clean && make ci`
- `git diff --check`

초기 `make ci`는 golangci-lint cache가 삭제된 sibling worktree를 참조해서 실패했다. lint cache를 정리하고 같은 명령을 다시 실행하자 통과했다.

## 이후 지침

- Redis-backed workshop 예제에서는 Testcontainers-backed 명령을 serial로 실행하고, package가 Pub/Sub 또는 lock connection을 즉시 열면 저렴한 client-level readiness check를 추가한다.
- 예제가 bluetape-go test helper를 import한다면 commit 전에 `go mod tidy`를 실행하고, issue가 명시적으로 허용하지 않는 한 `go.mod`에 추가 indirect requirement가 들어가지 않았는지 확인한다.
- README SVG route는 PNG를 승인하기 전에 단순 H/V segment crossing sweep을 실행한다. port나 corridor 변경으로 crossing을 피할 수 있다면, crossing을 허용 가능한 것으로 처리하지 말고 route를 고친다.
