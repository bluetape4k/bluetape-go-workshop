# Issue #44 ID/JWT Boundary Code Review

## Scope

- Branch: `feat/issue-44-id-jwt-boundary`
- Planning commit: `0303d19 Define the ID and JWT boundary before coding`
- Reviewed changed scope:
  - `examples/id-jwt-boundary/**`
  - `README.md`
  - `README.ko.md`
  - `go.mod`
  - `go.sum`
  - `docs/lessons/2026-06-22-id-jwt-boundary.md`

## Step 5 Verifier Result

PASS.

- Issue #44 acceptance criteria map to implementation:
  - ID generation: `Service.CreateOrder` uses an injected
    `id.NewUUIDV7Generator` default and tests parse IDs with `id.ParseUUID`.
  - JWT issue/verify: `/tokens` signs with `jwt.NewFixedHMACProvider`;
    `/orders` parses with expected issuer, audience, and expiration required.
  - Invalid token rejection: tests cover missing, malformed, wrong-key,
    expired, and forbidden-scope tokens.
  - Trust-boundary docs: example README pair states IDs are not bearer secrets
    and signed JWT claims are not encrypted.
  - No real secrets: the committed HMAC value is documented as deterministic
    demo material only.
  - Root navigation: `README.md` and `README.ko.md` include table and run
    section entries.

## Step 4-P Performance/Stability Scan

No performance or stability issues found in the reviewed scope.

| Priority | File:Line | Area | Finding | Fix |
|---|---|---|---|---|
| N/A | `examples/id-jwt-boundary/internal/idjwtboundary/service.go:265` | SECURITY/STABILITY | JSON request bodies are capped at 8 KiB before binding. | N/A |
| N/A | `examples/id-jwt-boundary/internal/idjwtboundary/service.go:262` | SECURITY | Gin trusted proxies are disabled for this local boundary example. | N/A |
| N/A | `examples/id-jwt-boundary/main.go:58` | OPS/STABILITY | HTTP server has bounded read-header, read, write, and idle timeouts. | N/A |
| N/A | `examples/id-jwt-boundary/main.go:76` | SECURITY | `HTTP_ADDR` rejects non-loopback binds by default. | N/A |
| N/A | `examples/id-jwt-boundary/internal/idjwtboundary/service.go:369` | SECURITY | Public error responses are allowlisted and do not include raw token/parser text. | N/A |

## Step 6-R Six-Lane Review

| Lane | P0 | P1 | P2 | P3 | Result |
|---|---:|---:|---:|---:|---|
| Tier 1 Performance | 0 | 0 | 0 | 0 | PASS |
| Tier 2 Stability | 0 | 0 | 0 | 0 | PASS |
| Tier 3 Security | 0 | 0 | 0 | 0 | PASS |
| Tier 4 Operator/Ops | 0 | 0 | 0 | 0 | PASS |
| Tier 5 Developer/API | 0 | 0 | 0 | 0 | PASS |
| Tier 6 User/Caller | 0 | 0 | 0 | 0 | PASS |

## Tier Notes

- Performance: request handling is bounded and local; no background workers,
  polling loops, sleeps, retries, or external services are introduced.
- Stability: deterministic tests inject clock, ID generator, and signing
  material. Entrypoint tests cover bind parsing and server timeouts.
- Security: error mapping collapses parser details into public codes; tests
  assert response bodies omit raw token, demo secret, and common parser text.
- Operator/Ops: loopback-only default keeps the unauthenticated demo API local;
  README documents production hardening boundaries.
- Developer/API: example-local `internal/idjwtboundary` owns the lesson without
  exporting reusable auth middleware or importing sibling `internal` packages.
- User/Caller: English/Korean README pair gives runnable curl flows for valid,
  missing, invalid, forbidden, and expired token paths.

## Production Boundary Quick Scan

Command:

```bash
rg -n "context\\.TODO\\(|httptest\\.NewRequest\\(|X-Forwarded-For|RealIP|ListenAndServe\\(|panic\\(|secret|token" examples/id-jwt-boundary README.md README.ko.md
```

Result: intentional hits only.

- `main.go`: owned `ListenAndServe` lifecycle and loopback bind validation.
- `service.go`: fixed HMAC demo secret, token issue/parse code, and allowlisted
  public token errors.
- `service_test.go`: context-aware `httptest.NewRequestWithContext`, test
  tokens, and leak assertions.
- README files: explicit warning that demo secret is not production material
  and JWT claims are not encrypted.

## Validation Evidence

- `go test -count=1 ./examples/id-jwt-boundary/...` -> PASS.
- `go test -race -count=1 ./examples/id-jwt-boundary/...` -> PASS.
- `go test -p 1 ./...` -> PASS.
- `make fmt-check` -> PASS.
- `make vet` -> PASS.
- `make lint` -> `0 issues.`
- `git diff --check` -> PASS.
- Live smoke:
  - `/healthz` -> 200 `{"status":"ok"}`
  - valid token order -> 201 with UUID v7 `order_id` and `request_id`
  - missing token -> 401 `missing_token`
  - malformed token -> 401 `invalid_token`
  - forbidden scope -> 403 `forbidden`
  - expired token -> 401 `expired_token`

## Convergence

Final gate: P0 = 0, P1 = 0.

No P2/P3 follow-up was identified in the final review scope.
