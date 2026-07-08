# Text Moderation Masking Lesson

Issue #53 is the first focused v0.8.0 text example. Keep the example local and
deterministic: application code owns the moderation policy, allowlist meaning,
and public decision contract, while `textsearch` owns exact phrase matching,
Unicode boundary filtering, NFC normalization, and deterministic spans.

The README needs to make exact search versus tokenization explicit. This example
does not choose language-specific tokens, call a model, or use a moderation
provider. It searches configured phrases, applies the Unicode word-boundary
helper, subtracts allowlisted spans, then masks only accepted match spans.

The minimum test suite for this lesson covers overlapping entries, Korean text,
allowlist subtraction, replacement boundaries, context cancellation, and preview
stability. The boundary test should include words such as `badge` and `badwolf`
so readers can see why substring replacement is not the intended contract.

Diagram QA lesson: avoid Korean text in SVG assets unless the rendered PNG font
stack is proven to include those glyphs. The first rendered sequence image
showed missing glyph boxes for Korean text; the final diagrams keep Korean
examples in README prose and use ASCII labels in the rendered assets.

Follow-up examples in the same v0.8.0 text track can add tokenizer selection,
language detection, HTTP routing, or larger content workflows, but they should
not blur the base contract: exact `textsearch` matching is a deterministic
helper, not a complete moderation or security boundary.
