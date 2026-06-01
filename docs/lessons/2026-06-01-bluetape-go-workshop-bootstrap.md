# bluetape-go-workshop Bootstrap

## Context

`bluetape-go` needs runnable web application examples without turning the
library repository into an application showcase.

## Decision

Create `bluetape-go-workshop` as the application example repository. Start with
one Redis leader web service and keep future examples tied to stabilized
`bluetape-go` milestones.

## Outcome

The repository has bilingual README files, local development commands, CI,
Nightly Testcontainers verification, and one runnable chi-based HTTP leader
example.

## Verification

- `actionlint .github/workflows/ci.yml .github/workflows/nightly-tests.yml`
- `golangci-lint config verify`
- `make ci`
- `git diff --check`

## Future Guard

Keep workshop examples thin. Application wiring belongs here, but reusable
support code should move back into `bluetape-go`.

Use `chi` as the default lightweight web framework for examples that need
routing or middleware while preserving `net/http` handler compatibility.
