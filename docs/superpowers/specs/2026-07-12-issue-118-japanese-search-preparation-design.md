# Issue #118 Japanese Search Preparation Design

## Status

Approved design for Issue #118, based on bluetape-go v0.18.0.

## Goal

Add an application-shaped command that prepares a small Japanese product
catalog for deterministic search. The command must expose Kagome search-mode
tokens, useful part-of-speech metadata, original UTF-8 byte spans, normalized
index terms, actual `textsearch` matching, and a support-text masking pass.

The example teaches how an application composes released packages. It does not
add reusable workshop infrastructure or copy package algorithms.

## Context

Issue #55 established the multilingual feasibility baseline and proved shared
detector/tokenizer reuse. Issue #118 narrows the lesson to Japanese product
search preparation and must precede the language-routing example in #119 and
the Gin integration example in #67.

The relevant v0.18.0 surfaces are:

- `japanese.NewTokenizer(japanese.WithMode(japanese.Search))` for reusable
  Kagome IPA search-mode tokenization;
- `japanese.IsNoun`, `japanese.IsVerb`, and Kagome metadata keys for selecting
  index candidates;
- `textsearch.Token` for original byte spans and normalized token text;
- `textsearch.Matcher` for deterministic matching over prepared index terms;
- `textsearch.BlockwordDictionary` for deterministic support-text masking.

## Chosen Approach

Use one local package with three explicit stages:

1. prepare products and query text with a shared Japanese tokenizer;
2. search an immutable in-memory prepared catalog with a query-local matcher;
3. mask configured support-text terms with a shared blockword dictionary.

This keeps the command cohesive while preserving boundaries between
morphological analysis, search policy, and masking policy. A single monolithic
method would obscure span ownership. Separate commands would duplicate fixture
and lifecycle setup and would not demonstrate the requested integration.

## Rejected Alternatives

### One monolithic service method

Rejected because token spans, prepared index positions, and masking spans have
different owners. Combining them in one output transformation would make it
easy for callers to slice the wrong text and difficult for tests to isolate a
failed contract.

### Separate search and masking commands

Rejected because both commands would repeat tokenizer and fixture lifecycle
setup. More importantly, the example would not prove how masking evidence can
exclude a token from an otherwise valid search projection.

### A generic workshop index abstraction

Rejected because Issue #118 needs one inspectable local catalog. A reusable
index interface would invent ranking, persistence, update, and concurrency
contracts that belong in a library or later application with multiple real
call sites.

## Package and Files

The implementation will live under:

```text
examples/japanese-search-preparation/
  main.go
  README.md
  README.ko.md
  internal/catalogprep/
    service.go
    service_test.go
```

The root `README.md` and `README.ko.md` will link the example. The umbrella
Issue #34 will mark completed child Issue #55 while Issue #118 remains open
until delivery.

## Domain Contract

### Inputs

`ProductInput` contains a caller-owned SKU, Japanese title, and Japanese
support text. All three values are required. `SearchRequest` contains a
non-blank Japanese query.

The example accepts only the input sizes supported by
`textsearch.NewTokenizeRequest`. It does not add a second size limit or silently
truncate input.

### Go API Shape

The internal example package exposes a constructor-only `Service`:

```go
func NewService(products []ProductInput, policy MaskPolicy) (*Service, error)
func (s *Service) Products() []PreparedProduct
func (s *Service) Search(request SearchRequest) (SearchResult, error)
func NewPreview() (Preview, error)
```

`DefaultProducts` and `DefaultMaskPolicy` provide the command fixtures. The
zero value of `Service` is intentionally unusable; every method checks the
receiver and returns `ErrInvalidService` rather than panicking. `Products`
returns a deep copy so callers cannot mutate the catalog or token metadata used
by concurrent searches.

### Prepared Tokens

Each selected token records:

- source field (`title` or `support_text`);
- original surface text;
- normalized text;
- base form when Kagome supplies one;
- full Kagome POS metadata;
- field-local, start-inclusive/end-exclusive UTF-8 byte offsets;
- whether the token is eligible for the search index.

Only nouns and verbs are selected. The index term is the NFC-normalized base
form when present and otherwise the normalized surface text. Duplicate terms
are removed while preserving first appearance. Title terms precede support
text terms.

Byte spans always slice the original field. They are never positions in the
masked projection or the prepared index string.

### Masking

A fixed local blockword policy detects configured unsafe terms in support text.
The service returns the original support text, a masked projection, and match
spans. A selected support token that overlaps a masked match remains visible as
evidence but is marked non-indexable and excluded from index terms.

The masking policy uses explicit substring semantics because Japanese text does
not reliably expose whitespace word boundaries. The README must identify this
as deterministic local policy rather than semantic moderation.

### Prepared Index and Search

Each product owns an inspectable index string made by joining its unique
indexable terms with spaces. Search prepares the query with the same tokenizer
and term projection, then compiles a query-local `textsearch.Matcher` using NFC
normalization and Unicode word boundaries over the space-delimited index.

A product is a hit only when every unique query term is present. Matched terms
are returned in query order. Hits are sorted by SKU so output is deterministic;
the example does not claim relevance ranking.

If a non-blank query yields no noun or verb terms, search returns an explicit
invalid-query error instead of matching every product.

## Service Lifecycle

`NewService` constructs:

- one Kagome IPA tokenizer in `japanese.Search` mode;
- one immutable blockword dictionary;
- one immutable prepared catalog.

The service reuses these values across calls. Query matchers and all mutable
result slices are local to one call. The service has no goroutines, channels,
I/O, timers, or shutdown responsibility. It therefore does not accept a
`context.Context`; adding one would imply cancellation behavior that this
CPU-only bounded fixture cannot honor meaningfully.

Kagome's IPA dictionary is an opt-in binary and memory cost already provided by
bluetape-go. The command constructs the tokenizer once and documents reuse; it
does not publish universal startup or memory measurements.

## Errors

The local package defines sentinel errors for invalid product and invalid
query input, plus invalid or nil service use. Constructors and methods wrap
causes with `%w` so callers can use `errors.Is` for both local sentinels and
upstream tokenization errors.

The following cases fail closed:

- blank SKU, title, or support text;
- blank query;
- input exceeding the upstream tokenization limit;
- a prepared query with no noun or verb terms;
- tokenizer, matcher, or dictionary construction failure.

The command treats these failures as startup/preview errors and exits non-zero.
No partial catalog is published when one fixture is invalid.

## Failure Modes and Guards

1. **A normalized term loses its original location.** Prepared tokens keep
   field-local source spans and tests slice the original field for every token;
   index-string positions are never exposed as source spans.
2. **A masked unsafe term leaks into search.** Mask matches are computed against
   original support text before index projection, and every overlapping token
   is retained as evidence but excluded from the index.
3. **Concurrent reuse mutates shared results.** The catalog, tokenizer, and
   dictionary are constructed once, while query matchers and result slices are
   call-local. Catalog access returns deep copies. Exact bounded stress and race
   tests guard this contract.
4. **Japanese substring behavior is mistaken for semantic moderation.** The
   output and README name the boundary policy explicitly and make no security,
   compliance, or semantic-safety claim.
5. **An empty prepared query accidentally matches the whole catalog.** Search
   rejects queries that yield no selected noun or verb before matcher creation.

## Command Output

`go run ./examples/japanese-search-preparation` prints deterministic indented
JSON containing:

- the scenario and tokenizer/masking lifecycle notes;
- prepared products with selected tokens, spans, POS, masked support text, and
  index terms;
- at least two fixed queries with expected matching and no-match outcomes;
- explicit boundary notes and validation commands.

The fixture remains small enough to inspect manually. Expected output snippets
in both README locales must use values generated by the command.

## Testing

Tests will be written before implementation and cover:

1. normal search-mode tokenization for title and support text;
2. noun/verb filtering and useful POS/base-form metadata;
3. NFC normalization while preserving original multibyte byte spans;
4. blank and oversized product/query failures with `errors.Is` preservation;
5. support-text detection, masking, match spans, and exclusion of overlapping
   terms from the index;
6. deterministic all-term matching, query-order matched terms, stable SKU
   ordering, and no-match results;
7. query text that produces no noun/verb term;
8. bounded shared-service reuse with exact completion/results under normal and
   race execution;
9. deterministic preview content used by the command and documentation.

The bounded stress test will share one service across a fixed worker/task set.
It will assert exact completed-call counts, exact hit SKUs, byte-span slicing,
and stable masking output. Both focused `go test` and `go test -race` are
required.

Repository validation remains:

```bash
make fmt-check
make tidy-check
make vet
make lint
make test
make race
make ci
```

Container-backed tests are not added by this issue. Existing repository-wide
Testcontainers tests still run sequentially through the normal `make ci` gate.

## Compatibility and Migration

This is a new standalone example. It does not change an existing workshop API,
wire format, or persisted value, so no caller migration is required. The root
README gains navigation only. The example consumes the already pinned
bluetape-go v0.18.0 dependency and adds no module requirement.

Future issues may borrow the demonstrated policy, but they must import
bluetape-go packages directly rather than treating this example's `internal`
package or JSON preview as a compatibility surface.

## Documentation

The example README pair will document:

- the package lesson and three-stage data flow;
- run and focused test commands;
- representative tokens, POS metadata, field-local byte spans, index terms,
  search hits, and masked support text;
- normal versus search-mode intent;
- Kagome IPA dictionary footprint and construct-once/reuse lifecycle;
- NFC normalization versus original span semantics;
- Japanese substring masking boundaries;
- unsupported non-Japanese tokenization, ranking, persistence, and HTTP paths.

The root README pair will add one navigation row and one compact run section.

## Non-goals

- No HTTP endpoint, database, durable index, ranking engine, stemming API, or
  generic search abstraction.
- No language detection or routing; Issue #119 owns that lesson.
- No second tokenizer, dictionary, or detector dependency.
- No rune-offset conversion; spans remain UTF-8 byte offsets.
- No claim that noun/verb selection, substring masking, or language-specific
  tokenization is a security, compliance, or semantic moderation decision.
- No duplication of `textsearch` matching, normalization, masking, or Kagome
  tokenization algorithms inside the workshop.

## Acceptance Mapping

| Issue requirement | Design evidence |
|---|---|
| Japanese tokenizer and application-owned lifecycle | One `japanese.Search` tokenizer constructed by `NewService` and reused. |
| Normalization, POS filtering, and multibyte spans | Prepared-token contract and field-local slicing tests. |
| Search/masking integration | Space-delimited prepared index plus query matcher; support dictionary with overlap exclusion. |
| Empty/invalid input | Local sentinels with upstream errors preserved through `%w`. |
| Concurrent reuse and race proof | Exact bounded stress assertions under focused normal/race tests. |
| Lifecycle and unsupported boundaries | Paired READMEs and deterministic preview notes. |
| Root navigation and repository quality | Root README pair and complete `make ci` gate. |

## Definition of Done

- The fixed command output demonstrates preparation, matching, masking, and
  their distinct span contracts.
- Every Issue #118 acceptance criterion maps to a deterministic normal,
  failure, edge, or race test.
- English and Korean README content remains synchronized and root navigation
  resolves to both locale files.
- Focused normal/race tests and repository `make ci` pass from a clean branch.
- Final review reports P0=0 and P1=0 with unsupported behavior documented.
- Delivery stops at a review-ready PR unless the user explicitly authorizes
  merge and local synchronization.
