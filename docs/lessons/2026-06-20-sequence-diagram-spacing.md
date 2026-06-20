# Sequence Diagram Spacing Lessons

## Context

Visual review found sequence diagrams where message labels overlapped nearby
message lines, adjacent labels, or the `alt failure / branch path` label.

## Decision

- Treat every message label rectangle as reserved space, not decoration.
- Keep standalone `alt` labels out of busy first-message corridors.
- For self-calls, route the loop outside the label rectangle; do not let the
  horizontal loop segment pass through its own label.
- Before accepting rendered PNGs, run a coordinate sweep for label-vs-line and
  label-vs-label overlaps across all sequence SVGs.

## Verification

- Re-render touched sequence SVGs with CairoSVG.
- Visually inspect a contact sheet and high-risk individual PNGs.
- Parse all sequence SVGs as XML.
- Run a sequence label spacing sweep across all README sequence SVGs.
