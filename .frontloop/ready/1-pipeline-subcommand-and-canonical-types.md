---
title: Add pipeline subcommand and canonical pipeline types
priority: high
---

## Goal

Create the first structural pieces for the new pipeline system without changing execution yet. Add a dedicated `juggle pipeline` subcommand and define the canonical internal types that will represent pipelines, nodes, agent specs, and command specs.

This task is about establishing the model and CLI entrypoint so later tasks can build on a stable shape.

## Acceptance Criteria

- `juggle pipeline` exists as a separate subcommand
- Running `juggle pipeline --help` shows pipeline-focused help text
- Canonical pipeline types exist in a dedicated package or module area suitable for reuse
- The type model covers:
  - pipeline metadata
  - node kind (`agent` or `cmd`)
  - shared orchestration fields
  - agent-specific execution fields
  - cmd-specific execution fields
- The model reflects the design in [docs/pipeline-design.md](/home/jmo/Development/active/tools/juggler/docs/pipeline-design.md)
- No lifecycle hook behavior is removed in this task
- Tests cover basic subcommand wiring and type-level invariants where appropriate

## Implementation Notes

- Keep the canonical model independent from current lifecycle hook structs
- Prefer a new `internal/pipeline` package over burying the scheduler model inside `internal/cli`
- The subcommand can be non-executing or stubbed initially if needed, but it should be wired cleanly

