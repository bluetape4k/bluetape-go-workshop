# leader-redis-web

Minimal chi-based HTTP service that uses `bluetape-go` Redis leader election.

## Run

```bash
export REDIS_ADDR=localhost:6379
go run ./examples/leader-redis-web
```

## API

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Health check. |
| `GET` | `/leader` | Current leader token and local leadership flag. |
| `POST` | `/campaign` | Try to acquire leadership. |
| `POST` | `/resign` | Release leadership when this instance owns it. |
