# Gin SQL order service diagram checklist repair

## Decision

Issue #65 README diagrams must be treated as source-backed teaching material,
not decorative assets. The architecture image is a static ownership map, while
the create-order and rollback images use the sequence-diagram family with
participants, lifelines, horizontal message lanes, branch regions, and rendered
PNG inspection.

## Rationale

The first version explained the right module boundaries, but the diagram pass
did not visibly prove the current `bluetape4k-diagram` checklist:

- sequence diagrams need local best-practices parity in the rendered PNG;
- rollback behavior is ordered behavior, so it should be a sequence diagram
  instead of a generic flowchart;
- marker, icon, connector, label, full-size PNG, and contact-sheet checks must
  be recorded as review evidence.

## Rule

For future README diagram corrections in this repository:

- read the target README and source implementation before drawing;
- compare sequence-named assets against nearby sequence family examples;
- render every touched SVG with CairoSVG before review;
- inspect every touched PNG at full size and also inspect a contact sheet when
  multiple diagrams change;
- record marker/icon/geometry audit evidence in `docs/review/` before PR
  creation.

