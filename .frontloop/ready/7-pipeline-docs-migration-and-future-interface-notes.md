---
title: Document pipeline usage, migration path, and future interfaces
priority: medium
---

## Goal

Turn the pipeline design into user-facing and developer-facing documentation that explains the new model, how it relates to existing lifecycle hooks, and how the file format positions Juggle for future GUI and round-trip workflows.

## Acceptance Criteria

- User-facing docs describe:
  - `juggle pipeline`
  - inline pipeline syntax
  - pipeline file usage
  - events
  - dependencies
  - conditions
- Docs explain how current lifecycle hooks map to pipeline concepts
- Docs explicitly preserve the future-facing design goals from [docs/pipeline-design.md](/home/jmo/Development/active/tools/juggler/docs/pipeline-design.md), including:
  - GUI pipeline editing
  - CLI/file/GUI round-tripping
  - canonical pipeline representation
- Help text or examples are updated where appropriate
- At least one migration-oriented example is added

## Implementation Notes

- Keep the design doc as the architectural reference
- README/FEATURES/help text should explain the shipped behavior, not every deferred idea
- Make sure the future GUI goal is documented as “same pipeline file, different frontend,” not a separate execution model
