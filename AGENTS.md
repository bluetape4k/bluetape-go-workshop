# AGENTS.md - bluetape-go-workshop

## Guidance hierarchy

Before applying this repository overlay, read and follow the guidance in this
order:

1. User scope: `${CODEX_HOME:-$HOME/.codex}/AGENTS.md`.
2. Workspace scope: `/Users/debop/work/bluetape4k/.github/docs/workspace/AGENTS.md`.

Apply both broader scopes before repository-specific rules.

This repository inherits the workspace guidance from `../AGENTS.md`.
Read and follow the workspace root guide first. This file only adds
Go-workshop layout, commands, domain rules, and local exceptions.

Runnable web application examples for `bluetape-go`.

## Skills

- Use `bluetape-workflow` for task classification and issue/PR discipline.
- Use `bluetape-go-patterns` for Go implementation, tests, examples, and review.

## Commands

```bash
make fmt-check
make tidy-check
make vet
make lint
make test
make race
make ci
go test ./...
```

## Rules

- Keep examples application-shaped; reusable library code belongs in
  `bluetape-go`.
- Prefer lightweight HTTP examples with `net/http`, `chi`, or `gin` only when
  the framework is part of the lesson.
- Every example README pair should show the package lesson, run command, and
  expected behavior.
- Container-backed integration examples should use existing `bluetape-go`
  Testcontainers fixtures and run sequentially when they share Docker resources.
