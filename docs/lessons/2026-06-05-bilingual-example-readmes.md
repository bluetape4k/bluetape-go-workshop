# Bilingual Example README Pass

## Context

The workshop examples had root-level English/Korean READMEs, but individual
example directories mostly had English-only documentation. The user asked for
English and Korean support across example READMEs and for the diagrams needed
to explain the scenarios.

## Decision

Add `README.ko.md` beside every example `README.md`, keep the surrounding prose
localized, and share English-label diagram assets across both locales. Rather
than create one diagram per language, each example points at a small set of
shared scenario diagrams under `docs/images/readme-diagrams/`.

## Outcome

- Root README files now include a workshop example map and locale links.
- Every example directory has both `README.md` and `README.ko.md`.
- Data/codec/cleanup, leadership, integration, and concurrency/resilience
  scenarios have PNG/SVG README diagram assets.
- Node-and-connector diagrams include DOT, Plain, and Graphviz render evidence.

## Verification

- Rendered every new diagram PNG from DOT.
- Inspected each new final PNG at readable size.
- Checked README embeds use PNG, not SVG.
- Checked new diagram SVGs do not use site UI font stacks.

## Future Guard

When adding a new workshop example, create `README.md` and `README.ko.md` in the
same change. If the README needs a diagram, embed the PNG in both locales and
keep the matching SVG plus structural evidence beside it.
