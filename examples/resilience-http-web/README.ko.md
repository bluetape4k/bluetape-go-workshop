# resilience-http-web

[English](README.md)

`bluetape-go/resilience`의 retry, timeout, circuit breaker, bulkhead, event hook을
조합하는 HTTP service 예제입니다.

## Scenario

![Concurrency and resilience flow](../../docs/images/readme-diagrams/concurrency-resilience-flow.png)

HTTP service가 outbound call과 inbound handler pressure에 서로 다른 policy를
적용해야 하는 상황을 보여줍니다. catalog request는 retry, timeout, circuit
breaker를 통과합니다. order creation은 reject-mode bulkhead로 보호되어 handler가
typed rejection을 빠르게 반환할 수 있습니다.

## Run

`:9090`에서 catalog service를 시작한 뒤 이 service를 실행합니다.

```bash
export CATALOG_URL=http://localhost:9090
go run ./examples/resilience-http-web
```

## API

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Health check. |
| `GET` | `/catalog/{id}` | retry, timeout, circuit breaker policy를 통해 catalog service를 호출합니다. |
| `POST` | `/orders?delay=25ms` | concurrent slot 하나를 가진 reject-mode bulkhead로 order를 처리합니다. |
| `GET` | `/events` | policy가 emit한 low-cardinality resilience event를 반환합니다. |

## What It Demonstrates

- outbound `net/http` call을 위한 retry + timeout + circuit breaker 조합.
- HTTP 5xx를 retryable `resilience.StatusError`로 변환.
- `errors.Is(err, resilience.ErrCircuitOpen)` 기반 circuit-open typed error handling.
- inbound handler를 위한 reject-mode bulkhead protection.
- `errors.Is(err, resilience.ErrBulkheadRejected)` 기반 bulkhead typed rejection handling.
- logging/metrics dependency 없이 synchronous event hook 사용.
