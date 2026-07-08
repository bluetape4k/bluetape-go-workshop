# text-moderation-masking

[English](README.md) | [한국어](README.ko.md)

Deterministic text moderation masking example for `textsearch`.

The example models a marketplace comment review pass. It does not call a model,
network service, or hidden moderation backend. Instead, the application owns the
policy dictionary and uses bluetape-go `textsearch` helpers for deterministic
multi-pattern search, Unicode word-boundary filtering, NFC normalization, and
masking.

This is the first focused text example in the v0.8.0 workshop track. Later
examples can add HTTP routing, tokenizer selection, language detection, and a
larger content workflow without hiding the basic matching contract.

## Scenario

The moderation pass receives one user comment and must:

1. compile a static blockword policy into an immutable dictionary;
2. find blockwords in mixed English/Korean text;
3. choose the leftmost-longest match when dictionary entries overlap;
4. skip matches that are fully contained in a known allowlist phrase;
5. mask only the exact accepted match spans;
6. report what was masked without leaking a separate provider or model.

The sample policy blocks `bad`, `bad wolf`, `scam`, `욕설`, and `무료 돈`. It also
allowlists the phrase `bad wolf book club` to show how application policy can
remove a known safe title before masking.

## Architecture

![Text moderation masking architecture](../../docs/images/readme-diagrams/text-moderation-masking-architecture.png)

The application owns the content review contract. `textsearch` owns only the
deterministic mechanics:

| Layer | Owns | Does not own |
|---|---|---|
| Caller | Content ID, raw comment text, request cancellation. | Dictionary compilation or masking internals. |
| Moderation service | Policy loading, allowlist subtraction, public decision, masked projection. | Tokenizer model choice, network moderation, storage. |
| `textsearch` | Immutable blockword dictionary, Unicode boundary checks, NFC matching, deterministic match spans. | Product policy, severity meaning, audit storage. |

## Processing Sequence

![Text moderation masking sequence](../../docs/images/readme-diagrams/text-moderation-masking-sequence.png)

The important distinction is exact search versus tokenization. This example
does not segment input into language-specific tokens. It searches configured
phrases and applies Unicode word-boundary filtering. That prevents examples
such as `badge` and `badwolf` from being masked by the `bad` rule, while still
masking standalone `bad` and Korean entries such as `욕설`.

## What It Demonstrates

- Overlapping entries select the leftmost-longest replacement span.
- Korean blockwords are matched and masked without a model dependency.
- Allowlist phrases can remove a contained match before masking.
- Unicode word boundaries avoid substring replacement inside larger words.
- Context cancellation is returned before policy processing starts.
- The service reports masked findings and allowlist hits separately.

## Run

Print the local preview:

```bash
go run ./examples/text-moderation-masking
```

The output is JSON:

```json
{
  "scenario": "review marketplace comments with deterministic blockword masking",
  "policy": {
    "boundary": "Unicode word boundary",
    "normalization": "NFC",
    "mask": "*"
  }
}
```

The full output includes three sample review results:

- `bad wolf offer와 욕설 포함` becomes `******** offer와 ** 포함`;
- `bad wolf book club discusses scam risk` preserves the allowlisted title and
  masks only `scam`;
- `badge bad badwolf bad.` masks only standalone `bad` occurrences.

## Test

Run the deterministic tests:

```bash
go test -count=1 ./examples/text-moderation-masking/...
go test -race -count=1 ./examples/text-moderation-masking/...
```

The tests prove overlapping pattern handling, Korean text masking, allowlist
subtraction, Unicode replacement boundaries, cancellation propagation, and
preview stability.

## Boundary Notes

- Blockword masking is a helper transform, not a complete moderation system.
- The example intentionally has no network or model dependency.
- `BoundaryUnicodeWord` is a false-positive reduction helper, not a security
  boundary.
- Tokenization and language detection are separate lessons in later v0.8.0
  examples.
