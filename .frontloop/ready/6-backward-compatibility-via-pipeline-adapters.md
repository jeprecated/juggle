---
title: Map existing lifecycle hooks onto the pipeline model
priority: medium
---

## Goal

Preserve current lifecycle hook behavior while unifying execution under the new pipeline abstraction. Existing hook flags should remain usable, but internally they should be representable as pipeline nodes.

## Acceptance Criteria

- Existing flags continue to work:
  - `--agent-pre`
  - `--agent-before`
  - `--agent-after`
  - `--agent-post`
  - `--cmd-before`
  - `--cmd-after`
- There is a clear adapter path from old lifecycle flags into the canonical pipeline model
- Behavior matches current semantics for:
  - ordering
  - failure handling
  - env propagation
  - watch mode behavior where applicable
- Existing lifecycle tests continue to pass or are updated without semantic regression
- New tests verify at least one compatibility mapping path explicitly

## Implementation Notes

- Do not remove the old user-facing flags in this task
- Keep the new pipeline engine as the primary abstraction, with old flags acting as sugar or translated config

