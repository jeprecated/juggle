---
title: Execute validated pipelines with event scheduling
priority: high
---

## Goal

Implement the first working pipeline executor. It should schedule validated nodes across the fixed event set and run `agent` and `cmd` nodes using the existing runner and shell-execution infrastructure where possible.

## Acceptance Criteria

- A validated pipeline can be executed end-to-end
- Supported events execute with the documented semantics:
  - `run-start`
  - `loop-start`
  - `loop-body`
  - `loop-end`
  - `run-end`
  - `failure`
- The scheduler respects:
  - dependency ordering
  - explicit `--parallel`
  - `--when` conditions
  - per-node failure policy
- `agent` nodes reuse existing provider/runner infrastructure as much as practical
- `cmd` nodes reuse or align with current shell hook execution behavior where sensible
- In v1, the single `loop-body` agent drives iterations
- Tests cover:
  - happy-path pipeline execution
  - per-event execution ordering
  - conditional execution
  - retry/continue/stop behavior
  - command-node execution
  - failure-event node execution

## Implementation Notes

- Start with deterministic scheduling even if concurrency is conservative
- If true parallel execution is too much for the first cut, the scheduler shape should still preserve room for it

