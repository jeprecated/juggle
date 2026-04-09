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
- Make sure the future GUI goal is documented as "same pipeline file, different frontend," not a separate execution model

## Completion Summary

Created `docs/pipeline.md` as the comprehensive user-facing and developer-facing pipeline reference covering:
- All 6 lifecycle events with a table
- Pipeline file format (TOML) with a full example
- Inline CLI syntax with an example
- Node kinds (`agent` and `cmd`) with all flags documented
- Shared node flags (event, after, parallel, when, on-failure, retries, timeout, workdir)
- Dependencies (implicit sequential, explicit --after, --parallel)
- Conditions (shell-command evaluation, JUGGLE_ITERATION usage)
- Failure policies (stop, continue, retry) with failure event nodes
- Migration from lifecycle hooks: table mapping each flag to pipeline equivalent
- Full before/after migration example (hook flags → pipeline file)
- Future GUI and round-tripping section framed as "same file, different frontend"

Updated `README.md`: added pipeline to the features bullet list.

Updated `FEATURES.md`: added a "Pipeline Model" section at the end explaining the internal scheduler model, the hook-to-pipeline mapping table, and a link to docs/pipeline.md.

Files changed: `docs/pipeline.md` (created), `README.md`, `FEATURES.md`.
