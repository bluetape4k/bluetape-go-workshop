# Issue #47 Research: Graph and Text Workshop Candidates

## Scope

- Repository: `bluetape4k/bluetape-go-workshop`
- Issue: #47 `[v0.7.0] Research graph and text workshop candidates`
- Parent track: #31 `[v0.7.0] Plan post-utility workshop tracks`
- Parent roadmap epic: #27
- Milestone: `0.7.0`
- Work type: Type E - Research / Maintenance

## Sources Checked

- GitHub issue #47, parent #31, and follow-up workshop issues:
  - Text: #34, #53, #54, #55, #67
  - Graph: #36, #50, #51, #52, #69
- Upstream bluetape-go research:
  - `/Users/debop/work/bluetape4k/bluetape-go/docs/research/2026-06-01-milestone-0.10.0-text-research.md`
  - `/Users/debop/work/bluetape4k/bluetape-go/docs/research/2026-06-01-milestone-0.12.0-graph-research.md`
- Kotlin workshop examples:
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/kotlin/text-processing/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/spring-data/elasticsearch/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/graph/abuser-detection/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/graph/recommendation/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/graph/social-network/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-workshop/graph/knowledge-graph/README.md`
- Upstream Kotlin package references:
  - `/Users/debop/work/bluetape4k/bluetape4k-text/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-text/text-search/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-text/tokenizer-japanese/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-text/lingua/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-graph/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-graph/graph/graph-core/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-graph/graph-io/core/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-graph/graph-io/csv/README.md`
  - `/Users/debop/work/bluetape4k/bluetape4k-graph/graph-io/graphml/README.md`
- Context-mode search for issue #47 / #31 history and current issue bodies.

## Current Evidence

Issue #47 asks for candidate selection before implementation. Its suggested
scope still says the examples should target 0.8.0 and 0.9.0, but the current
parent track #31 maps text to 0.10.0 (#34) and graph to 0.12.0 (#36). This
research follows the current parent mapping and treats the older 0.8.0/0.9.0
sentence as stale issue text.

The upstream bluetape-go text research favors deterministic search and masking
first. It explicitly keeps full Korean/Japanese tokenizer adoption behind
dependency, dictionary, license, and binary-size review. The upstream graph
research favors one useful domain workflow plus portable import/export before
any broad backend abstraction.

The Kotlin text examples show abuse-word filtering, language detection, and
normalization as the strongest workshop-shaped lessons. The Kotlin text-search
package highlights Aho-Corasick multi-keyword search, Unicode normalization,
word-boundary behavior, case-insensitive search, tokenization helpers, Flow
APIs, profanity masking, and Korean normalization. The Lingua and Japanese
tokenizer packages are valuable references, but they are dependency-heavy
relative to the first Go workshop examples.

The Kotlin graph workshop has several domain examples. `abuser-detection`
models users, devices, IPs, payment tokens, and suspicious clusters, then ranks
risky users. `recommendation` models products, purchases, and follows for
collaborative filtering and friend-of-friend recommendations. `social-network`
overlaps the recommendation shape. `knowledge-graph` is useful but broader and
more abstract than the first graph workshop pass. The graph package references
also show graph I/O, traversal, path, component, cycle, and import/export
contracts that should remain visible instead of being hidden behind a generic
service layer.

## Accepted Text Candidates

### #53 Text Moderation Masking

Accept as the first text workshop example.

Reasons:

- It maps directly to `bluetape4k-text/text-search` search and masking behavior.
- It also maps to the Kotlin workshop `kotlin/text-processing` abuse-word
  filtering lesson.
- It can stay deterministic, local, and dependency-free.
- It lets the workshop cover overlapping patterns, mixed Korean/English input,
  replacement boundaries, allowlists, and visible unsupported cases without a
  model or network dependency.

Implementation boundary:

- Build the domain package before any HTTP wrapper.
- Keep the masking policy explicit and testable.
- Do not introduce a production tokenizer dependency in this issue.

### #54 Gin Text Search Service

Accept after #53 provides the domain behavior.

Reasons:

- It is the smallest public API example for the text track.
- It keeps handlers thin while exposing search/mask behavior through a workshop
  service boundary.
- It can document Unicode and normalization caveats without promising full
  language coverage.

Implementation boundary:

- Use Gin because the workshop issue asks for a public HTTP API example.
- Keep HTTP validation and response projection separate from search/masking
  logic.

### #55 Tokenizer / Language Detection Feasibility

Accept as a feasibility and contract example, not as a dependency-heavy
production tokenizer port.

Reasons:

- The Kotlin references show useful tokenizer, language detection, and mixed
  language behavior.
- The upstream Go research keeps full tokenizer adoption gated by dependency
  review.
- The workshop can still teach explicit handling for unknown, ambiguous, and
  mixed-language input.

Implementation boundary:

- Start with deterministic heuristics, fixture-driven tests, and clear
  limitations.
- Add any external tokenizer or language detector only after a separate
  dependency decision.

### #67 Gin Content Moderation Workflow Integration

Accept as the text track integration issue after #53, #54, and #55 are in
place.

Reasons:

- It composes the focused examples into one realistic content moderation flow.
- It can show search, masking, token/language classification, and unsupported
  language handling in one API.
- It is scenario-shaped and avoids becoming a thin package API tour.

Implementation boundary:

- Link back to #53, #54, and #55 from the README.
- Keep moderation decisions deterministic and local.

## Deferred or Rejected Text Candidates

- Reject a Spring Data Elasticsearch port for #47. The source example is useful
  search infrastructure evidence, but it is external-service and Spring-shaped,
  while #34/#53/#54/#55/#67 target local bluetape-go text behavior first.
- Defer full Korean/Japanese production tokenizer examples. The package value
  is real, but dependency, dictionary, license, and binary-size risks need a
  separate decision before workshop code depends on them.
- Reject generic text framework scaffolding. The workshop needs scenario-shaped
  examples with observable behavior, not an abstraction layer.

## Accepted Graph Candidates

### #50 Graph Abuse Cluster

Accept as the first graph workshop domain example.

Reasons:

- It maps directly to the Kotlin `graph/abuser-detection` example.
- The domain is concrete: users, devices, IPs, payment tokens, and shared
  identifiers.
- It exercises traversal and cluster reasoning without requiring a broad graph
  backend abstraction.
- It naturally teaches security boundaries because sensitive identifiers can be
  represented as hashed or opaque values.

Implementation boundary:

- Use a small deterministic fixture and expected cluster output.
- Keep identifiers opaque in README and tests.
- Avoid multi-backend adapters unless the upstream Go graph API already makes
  one stable backend trivial.

### #51 Graph Recommendation

Accept as the second graph domain example.

Reasons:

- It maps to Kotlin `graph/recommendation` and partially reuses the
  `graph/social-network` friend-of-friend idea.
- Ranking and tie-breaking are easy to test deterministically.
- The domain is familiar enough for a workshop while still proving traversal
  and neighborhood reasoning.

Implementation boundary:

- Model users, products, purchases, and follows with a small fixture.
- Keep recommendation scoring local and transparent.
- Test ranking, tie-breaking, and empty recommendation cases.

### #52 Graph Import / Export Fixtures

Accept after the upstream Go graph I/O helpers are available.

Reasons:

- It maps to graph I/O references for core records, CSV, and GraphML.
- It gives later graph examples reusable fixture import/export behavior.
- It is valuable only if it stays small and reviewable.

Implementation boundary:

- Use tiny CSV, NDJSON, or GraphML fixtures.
- Prove duplicate and missing-endpoint handling if the upstream API exposes
  those policies.
- Do not build a custom workshop-only graph I/O layer if the upstream package
  surface is not ready.

### #69 Graph Risk Intelligence Integration

Accept as the graph track integration issue after #50, #51, and #52.

Reasons:

- It combines import/export, traversal, scoring, and reporting in a realistic
  risk workflow.
- It can link back to the focused graph examples instead of re-teaching every
  primitive.
- It fits the milestone goal of scenario-shaped graph workshops.

Implementation boundary:

- Keep risk scoring deterministic and explainable.
- Use only the graph package surfaces already proven by the focused examples.

## Deferred or Rejected Graph Candidates

- Reject a broad backend abstraction example for #47. The upstream graph
  research warns against hiding Cypher, Gremlin, or backend-specific behavior
  behind a leaky generic query API too early.
- Defer multi-backend Testcontainers matrices. They are useful for upstream
  package confidence but too heavy for the first Go workshop graph examples.
- Reject a direct Ktor or Spring graph service port. The Go workshop should use
  Gin or plain HTTP only when the issue needs an HTTP boundary.
- Defer a standalone knowledge-graph example. It is useful but broader and less
  immediate than abuse-cluster and recommendation. Revisit it after #50/#51/#52
  prove the graph package surface.
- Reject bare traversal snippets. Issue #36 asks for domain graph scenarios,
  not isolated API demonstrations.

## Recommended Sequence

1. Implement #53 before the rest of the text track.
2. Implement #54 as the public API wrapper over the text behavior.
3. Use #55 to decide whether tokenizer/language detection stays heuristic or
   adopts a dependency.
4. Implement #67 as the text integration example.
5. Implement #50 before the rest of the graph track.
6. Implement #51 as the second graph domain example.
7. Implement #52 when upstream graph I/O is stable enough.
8. Implement #69 as the graph integration example.

## DoD Evidence For #47

- Concrete source paths are listed in `Sources Checked`.
- Accepted examples are listed with reasons and follow-up issue links:
  - Text: #53, #54, #55, #67
  - Graph: #50, #51, #52, #69
- Rejected and deferred candidates are listed with reasons.
- The stale 0.8.0/0.9.0 wording in #47 is resolved against current parent #31
  milestone mapping.
