---
title: Add canonical pipeline file format and loader
priority: high
---

## Goal

Implement a persisted pipeline file format and loader so pipelines can be stored, reviewed, edited, and later used by a GUI. The file format should be the canonical representation, with inline CLI acting as another frontend to the same model.

## Acceptance Criteria

- `juggle pipeline` supports loading a pipeline from file
- The file format matches the design in [docs/pipeline-design.md](/home/jmo/Development/active/tools/juggler/docs/pipeline-design.md)
- The format supports:
  - top-level pipeline metadata
  - default agent settings
  - `[[agent]]` nodes
  - `[[cmd]]` nodes
- File loading produces the same canonical pipeline model used by the inline parser
- Invalid files return actionable validation or parse errors
- At least one example file is added to tests or fixtures
- Tests cover:
  - valid file loading
  - mixed node kinds
  - default inheritance where applicable
  - invalid file structure

## Implementation Notes

- Prefer TOML to align with existing config usage in the repo
- Keep the loader focused on decoding and normalization, not execution
- Preserve the long-term possibility of optional GUI metadata in the file format

