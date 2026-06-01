# bluetape-go-workshop

[English](README.md) | [한국어](README.ko.md)

[`bluetape-go`](https://github.com/bluetape4k/bluetape-go)를 실제 웹 애플리케이션 형태로 사용하는 예제 저장소입니다.

이 저장소는 재사용 가능한 library package와 runnable application 예제를 분리하기
위해 둡니다. Library 저장소는 안정적인 package에 집중하고, workshop 저장소는
그 package들이 HTTP service와 container-backed integration test 안에서 어떻게
동작하는지 보여줍니다.

기본 web style은 lightweight 방향으로 둡니다. Routing과 middleware가 필요할 때는
[`chi`](https://github.com/go-chi/chi)를 사용하되, handler는 `net/http`와
호환되게 유지합니다.

## 예제

| 예제 | 목적 | bluetape-go package |
|---|---|---|
| [`examples/leader-redis-web`](examples/leader-redis-web) | Redis 기반 leader election을 수행하고 leader 상태를 노출하는 최소 chi 기반 HTTP service입니다. | `leader`, `leader/redis`, `testcontainers/redis` |

## Leader 예제 실행

Redis를 먼저 실행한 뒤 service를 시작합니다:

```bash
export REDIS_ADDR=localhost:6379
go run ./examples/leader-redis-web
```

주요 endpoint:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/leader
curl -X POST http://localhost:8080/campaign
curl -X POST http://localhost:8080/resign
```

## 개발

자주 쓰는 명령:

```bash
make ci
```

| 명령 | 설명 |
|---|---|
| `make fmt` | Go source를 `gofmt`로 format합니다. |
| `make fmt-check` | Go source가 `gofmt` 형식이 아니면 실패합니다. |
| `make tidy-check` | `go mod tidy` 후 `go.mod`/`go.sum` 변경이 있으면 실패합니다. |
| `make vet` | `go vet ./...`를 실행합니다. |
| `make lint` | `golangci-lint run ./...`를 실행합니다. |
| `make test` | Testcontainers 테스트가 실제 실행되도록 `go test -count=1 ./...`를 실행합니다. |
| `make race` | Testcontainers 테스트가 race detector에서도 실제 실행되도록 `go test -race -count=1 ./...`를 실행합니다. |
| `make ci` | 로컬 CI gate를 실행합니다. |

Integration test는 Testcontainers를 사용하므로 Docker가 필요합니다. 일반 CI와
Nightly workflow 모두 실제 container를 사용해 테스트합니다.

## Roadmap

| bluetape-go milestone | Workshop 예제 방향 |
|---|---|
| `0.1.0` | Redis leader election web service. |
| `0.2.0` | HTTP client/service resilience 예제. |
| `0.3.0` | Near-cache와 Redis invalidation 예제. |
| `0.4.0` | State와 workflow 예제. |
| `0.5.0` | Batch processing 예제. |
