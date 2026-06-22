# Issue #47 Review: Graph and Text Candidate Research

## Scope

- Reviewed artifact:
  `docs/superpowers/research/2026-06-23-issue-47-graph-text-candidates-research.md`
- Issue: #47
- Milestone: `0.7.0`
- Review type: documentation / research acceptance review

## Checks

- Source-path evidence is present for workshop, upstream Go research, Kotlin
  text, and Kotlin graph references.
- Accepted examples include follow-up issue links:
  - Text: #53, #54, #55, #67
  - Graph: #50, #51, #52, #69
- Rejected and deferred candidates include rationale.
- The stale #47 0.8.0/0.9.0 wording is reconciled against the current #31
  mapping of text to 0.10.0 and graph to 0.12.0.
- No implementation, dependency, README navigation, or generated artifact
  changes are included in this research-only PR.

## Findings

No P0/P1 findings.

## Verification

```bash
rg -n "#5[0-5]|#67|#69|Sources Checked|Accepted|Rejected|Deferred|0\\.10\\.0|0\\.12\\.0" docs/superpowers/research/2026-06-23-issue-47-graph-text-candidates-research.md
git diff --check
```
