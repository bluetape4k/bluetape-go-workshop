# Sequence Diagram Spacing

## Context

Several README sequence diagrams had call labels close enough to the next call
line or branch-region label that the rendered PNG looked cramped.

## Decision

Fix sequence spacing by increasing vertical row gaps and the SVG canvas height
together. Do not use lateral label movement as the primary fix for call-to-call
or alt-label collisions, because it hides the symptom while preserving cramped
message lanes.

## Future Guidance

- When call labels overlap the next call line, move the later message rows down.
- Increase `height`, `viewBox`, canvas height, lifeline length, activation
  height, alt frame height, and footer position by the same effective delta.
- Keep participant positions and label `x` coordinates stable unless the source
  or target participant order is wrong.
- Re-render PNGs with CairoSVG and run an id-aware sweep for label-label and
  label-line overlaps before committing sequence diagram changes.
