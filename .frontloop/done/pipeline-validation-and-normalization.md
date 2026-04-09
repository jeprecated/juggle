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
- This task is where the "default to previous node" rule should be encoded explicitly

## Completion Summary

Added `internal/pipeline/validate.go` with `Normalize(*Pipeline) error` — a single function
covering both normalization and validation called by `runPipelineCmd` in `internal/cli/pipeline.go`.

**Normalization**: walks nodes in order; if a node has no explicit `After` and is not `Parallel`,
sets `After = [prevNode.Name]`.

**Validation** (in order):
1. Unique node names
2. Kind payload consistency (agent→AgentSpec, cmd→CmdSpec)
3. Event name validity (when non-empty)
4. After targets exist and are not forward references
5. DFS cycle detection
6. v1 invariant: exactly one `agent` node with `event=loop-body`

Tests in `internal/pipeline/validate_test.go` cover all acceptance criteria (16 test functions).
All 873 tests pass.
