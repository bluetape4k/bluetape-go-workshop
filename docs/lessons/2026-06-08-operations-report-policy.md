# Lesson: Operations Report Policy Example

## Change

- Added `examples/operations-report-policy` with a runnable Gin main package.
- Added a deterministic report DTO that omits `workreport.Report` runtime
  timestamps.
- Demonstrated `StopOnFailure` and `ContinueOnFailure` using direct
  `workreport.Aggregate` calls.
- Added retry evidence as nested reports without marking preserved failed
  attempts as full success.
- Added skipped work representation with `aborted` plus a caller-defined reason.
- Added English/Korean README files and diagram assets for scenario,
  architecture, and sequence sections.

## Guardrails

- Keep `workreport` examples honest: a retry aggregate with a preserved failed
  attempt should remain `partial`.
- Keep API responses deterministic by projecting reports into DTOs instead of
  exposing `StartedAt` or `EndedAt`.
- Use `aborted` with a reason for caller-skipped work unless the library adds a
  first-class skipped status.
- Do not add persistence, background workers, queues, or observability
  dependencies to this focused example.

## Validation

- `bash scripts/generate-operations-report-policy-diagrams.sh`
- visual inspection of:
  - `operations-report-policy-scenario.png`
  - `operations-report-policy-architecture.png`
  - `operations-report-policy-sequence.png`
  - `workshop-example-map.png`
- `go test -count=1 ./examples/operations-report-policy/...`
- `go test -race -count=1 ./examples/operations-report-policy/...`
- `go test -run '^$' ./examples/operations-report-policy`
