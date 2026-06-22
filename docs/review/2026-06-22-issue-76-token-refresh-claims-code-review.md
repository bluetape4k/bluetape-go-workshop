# Issue #76 Token Refresh Claims Code Review

## Scope

- Branch: `feat/issue-76-token-refresh`
- Issue: #76 `[v0.6.0] Add token refresh and claims validation example`
- Planning commit: `0000001 Define the token refresh claims boundary`
- Reviewed files:
  - `examples/token-refresh-claims/**`
  - `README.md`
  - `README.ko.md`
  - `docs/lessons/2026-06-22-token-refresh-claims.md`

## Findings

No P0/P1 findings.

## Acceptance Evidence

- Runnable example: `examples/token-refresh-claims/main.go` wires a loopback
  Gin API with `/healthz`, `/sessions`, `/profile`, and `/tokens/refresh`.
- Valid claims: `/profile` accepts access tokens with issuer, access audience,
  `token_use=access`, role, scope, subject, and `session_id`.
- Expired tokens: tests compose expired access and refresh tokens and assert
  `expired_token`.
- Invalid claims: tests cover wrong audience, wrong `token_use`, missing scope,
  refresh-as-access, and access-as-refresh.
- Refresh behavior: valid refresh token returns a new access token accepted by
  `/profile`.
- README linkage: English/Korean root and example READMEs link #44
  `examples/id-jwt-boundary` as the base boundary lesson.

## Six-Lane Review

| Lane | P0 | P1 | P2 | P3 | Result |
|---|---:|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | 0 | PASS |
| Stability | 0 | 0 | 0 | 0 | PASS |
| Security | 0 | 0 | 0 | 0 | PASS |
| Operator/Ops | 0 | 0 | 0 | 0 | PASS |
| Developer/API | 0 | 0 | 0 | 0 | PASS |
| User/Caller | 0 | 0 | 0 | 0 | PASS |

## Review Notes

- Performance: token issue and parse work is request-local. The example adds no
  background workers, stores, network clients, sleeps, or retry loops.
- Stability: tests inject clock, secret, and deterministic ID generation. HTTP
  body size is capped at 8 KiB before JSON binding.
- Security: access and refresh tokens are separated by audience and
  `token_use`; public error responses are allowlisted and tests assert no raw
  token, demo secret, or parser diagnostic leaks.
- Operator/Ops: the entrypoint rejects non-loopback `HTTP_ADDR` values and uses
  bounded read-header, read, write, and idle timeouts.
- Developer/API: the example stays in an `internal/tokenrefresh` package and
  uses existing `bluetape-go/jwt` APIs directly.
- User/Caller: README flows cover session issue, protected profile, refresh
  exchange, and boundary failures.

## Verification

```bash
go test -count=1 ./examples/token-refresh-claims/...
go test -race -count=1 ./examples/token-refresh-claims/...
go test -p 1 ./...
make fmt-check
make tidy-check
make vet
make lint
GOFLAGS=-p=1 make ci
git diff --check
rg -n "context\\.TODO\\(|httptest\\.NewRequest\\(|X-Forwarded-For|RealIP|ListenAndServe\\(|panic\\(|secret|token" examples/token-refresh-claims README.md README.ko.md
```

Result:

- `go test -count=1 ./examples/token-refresh-claims/...` -> PASS.
- `go test -race -count=1 ./examples/token-refresh-claims/...` -> PASS.
- `go test -p 1 ./...` -> PASS.
- `make fmt-check` -> PASS.
- `make tidy-check` -> PASS.
- `make vet` -> PASS.
- `make lint` -> PASS after `golangci-lint cache clean` removed stale paths
  from deleted issue #44 worktree.
- `GOFLAGS=-p=1 make ci` -> PASS.
- `git diff --check` -> PASS.
- `rg` review found expected token/secret documentation and test fixtures only;
  no `panic`, forwarded-header trust, or unbounded server construction issue.
- Live smoke with `go run ./examples/token-refresh-claims` -> PASS for
  `/healthz`, `/sessions`, access-token `/profile`, refresh exchange, refreshed
  `/profile`, and refresh-as-access `403 invalid_claims`.

## Residual Risk

- Refresh-token reuse detection is intentionally out of scope. README documents
  durable session/revocation storage and replay monitoring as production
  hardening.
