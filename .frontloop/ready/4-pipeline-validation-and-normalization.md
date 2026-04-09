---
title: Validate and normalize pipelines before execution
priority: high
---

## Goal

Add a validation and normalization pass for parsed pipelines so both inline CLI and file-loaded pipelines share one consistent set of rules before scheduling.

This includes implicit dependencies, event validation, and v1 invariants.

## Acceptance Criteria

- A single validation/normalization path exists for both inline and file pipelines
- If `--after` is omitted and `--parallel` is not set, the node implicitly depends on the previous node
- `--parallel` suppresses the implicit previous-node dependency
- Validation enforces:
  - unique node names
  - valid node kind payloads
  - valid event names
  - `--after` targets exist
  - no forward dependencies
  - no dependency cycles
  - exactly one `agent` node with `event=loop-body` in v1
- Validation errors are specific and readable
- Tests cover:
  - implicit dependency insertion
  - explicit dependency replacement
  - parallel nodes
  - cycle detection
  - duplicate names
  - missing loop-body node
  - multiple loop-body nodes

## Implementation Notes

- Keep normalization separate from execution so dry-run, export, and future GUI use the same resolved graph
- This task is where the “default to previous node” rule should be encoded explicitly

