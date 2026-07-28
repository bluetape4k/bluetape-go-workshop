# ID와 JWT Boundary 예제 Lesson

## 맥락

- Issue: #44 `[v0.6.0] Add ID and JWT boundary example`
- Example: `examples/id-jwt-boundary`
- Packages: `github.com/bluetape4k/bluetape-go/id`,
  `github.com/bluetape4k/bluetape-go/jwt`

## 결정

- reusable auth middleware가 아니라 작은 Gin order intake API로 예제를 유지한다. lesson은
  framework가 아니라 application boundary다.
- deterministic local command와 test를 위해서만 committed demo secret과
  `jwt.NewFixedHMACProvider`를 사용한다.
- production wiring에는 `id.NewUUIDV7Generator`를 사용하고, deterministic service test에는
  injected generator를 사용한다.
- upstream JWT parse failure는 allowlisted public code로 mapping한다. expired token은
  `expired_token`, signature/malformed/key/issuer/audience failure는 `invalid_token`을
  받는다.
- 코드와 문서에서 UUID v7 값과 JWT bearer token은 개념적으로 분리한다. ID는 credential이
  아니고 signed claim은 encrypted data가 아니다.

## 주의점

- `bluetape-go/jwt`와 `bluetape-go/id`를 처음 import할 때 `go mod tidy`가
  `github.com/golang-jwt/jwt/v5`, `github.com/oklog/ulid/v2`,
  `github.com/segmentio/ksuid`의 transitive `go.sum` checksum을 추가해야 했다.
- expired-token regression test는 service provider와 같은 `kid`로 sign해야 한다. 그렇지
  않으면 key lookup이 먼저 실패하고, 올바르게 `invalid_token`으로 mapping된다.
- JWT library는 유용하지만 boundary에 부적절한 error text를 자주 반환하므로 public error
  test는 raw token, demo secret, 흔한 parser diagnostic이 없음을 assert해야 한다.

## 검증 메모

- focused service test는 valid, missing, expired, malformed, wrong-key, forbidden,
  invalid request, ID shape, ID generator failure path를 다룬다.
- entrypoint test는 loopback-only bind resolution과 HTTP server timeout을 다룬다.
- live smoke에는 다음이 포함되어야 한다.
  - `/healthz`
  - `/tokens`
  - valid token으로 호출한 `/orders`
  - missing token
  - malformed token
  - forbidden scope
  - expired token

## Follow-Up

- Issue #76은 더 명시적인 identifier/key tradeoff 비교를 추가하기 전에 이 예제를 base
  boundary로 link할 수 있다.
- Issue #78은 auth를 다시 구현하지 않아야 한다. 명확히 문서화된 trust boundary를 통해 이
  예제를 이후 storage/cache/realtime example과 조합한다.

## Diagram Evidence 검증

- scenario, architecture, protected-order sequence는 현재 example source에 근거하며 두
  README locale에서 같은 asset order로 공유된다.
- SVG-to-PNG review는 rendered arrowhead direction, explicit per-color marker parity,
  native pixel 기준 intended boundary와 endpoint contact, full-size readability를 검증해야
  한다. valid SVG만으로는 evidence가 아니다.
- connector bend는 marker footprint를 위한 최소 24 px straight terminal distance와,
  arrowhead가 corner에 붙지 않도록 충분한 bend clearance를 예약한다. automated geometry
  success만으로는 충분하지 않다.
- JWT failure는 UUID generation 전에 명확히 종료되므로 diagram은 identifier가 request를
  authorize한다고 암시하면 안 된다.
