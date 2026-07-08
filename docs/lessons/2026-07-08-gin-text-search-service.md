# Gin Text Search Service Lesson

Issue #54 should stay one step above the exact matching example and one step
below the later workflow integration. The useful teaching boundary is: Gin owns
HTTP parsing and public error shape, while a reusable service owns policy
compilation, Unicode boundary matching, overlap handling, and masking.

Keep the endpoint scenario-shaped. A single `POST /text/search-mask` endpoint is
enough when the response includes matches, summary counts, masked text, and
Unicode caveats. Splitting the demo into unrelated search and mask endpoints
would make handlers look more important than the shared domain contract.

Tests should prove both sides of the boundary:

- HTTP tests assert status codes, JSON field names, error codes, and that
  handlers remain thin delegates.
- Domain tests assert Korean text, overlap handling, word-boundary behavior, and
  exact original-span masking.

No Testcontainers run is required for this issue because the acceptance criteria
do not involve persistence, queues, external services, or Docker-backed
protocol compatibility. The right validation is deterministic Go tests, race
tests, example execution, and rendered README diagram inspection.

Diagram QA lesson: HTTP diagrams become hard to read when route labels share a
short connector corridor with card titles. Put long route labels in their own
small pill away from card headings, render the PNG, and inspect it before PR.
