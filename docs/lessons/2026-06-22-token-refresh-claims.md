# Token Refresh Claims 예제 Lesson

## 맥락

- Issue: #76 `[v0.6.0] Add token refresh and claims validation example`
- Example: `examples/token-refresh-claims`
- Dependency focus: `github.com/bluetape4k/bluetape-go/jwt`
- Base lesson: #44 `examples/id-jwt-boundary`

## Lesson

- access-token과 refresh-token contract를 명시적으로 유지한다. 이 예제는 서로 다른 `aud`
  값과 `token_use` claim을 사용하므로 검증된 refresh token도 protected resource에서는
  거부된다.
- signature, issuer, expiration을 검증할 만큼 token을 parse한 뒤 operation-specific claim
  mismatch를 local `invalid_claims` public code로 mapping한다. 이렇게 하면 wrong audience나
  wrong token-use를 malformed 또는 unverifiable token과 분리할 수 있다.
- stateless refresh exchange는 workshop boundary에는 유용하지만 production session
  management로 설명하면 안 된다. durable session storage, revocation, reuse detection,
  key rotation, TLS, log scrubbing은 production responsibility로 남는다.
- public error response는 allowlisted 상태로 유지한다. raw bearer value, demo secret,
  parser diagnostic은 test와 log에는 유용하지만 caller에게 적합하지 않다.

## 검증 명령

```bash
go test -count=1 ./examples/token-refresh-claims/...
go test -race -count=1 ./examples/token-refresh-claims/...
```
