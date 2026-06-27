# Code review: issue #111 distributed JWT key rotation

## Scope

- New runnable example: `examples/distributed-jwt-key-rotation`
- Root README navigation updates
- Lesson note for the distributed JWT/Redis example

## Findings

No P0/P1 findings in the local review pass.

## Checks

- Service methods create bounded child contexts before repository-backed JWT
  operations.
- Public errors do not leak token strings or Redis internals.
- Cache usage stays behind `jwt.NewCachedDistributedProvider`, which revalidates
  retained `kid` state before returning warm readers.
- Tests cover shared-key verification, forced rotation, unknown `kid`, expired
  token, cancellation, and warm cached verification after rotation.

## Residual risk

The example intentionally demonstrates HMAC key rotation only. RSA and MongoDB
distributed repositories are left for focused follow-up examples if the 0.6.x
milestone needs broader repository coverage.
