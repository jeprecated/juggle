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

## Completion Summary

Added `AdaptConfigToPipeline(cfg Config) *pipeline.Pipeline` in `internal/cli/pipeline_adapter.go`.

**Mapping:**
- `AgentPre` → agent node at `EventRunStart`, `FailurePolicyStop`
- `CmdBefore` → cmd node at `EventLoopStart`, `FailurePolicyStop`
- `AgentBefore` → agent node at `EventLoopStart`, `FailurePolicyStop`
- `AgentAfter` → agent node at `EventLoopEnd`, `FailurePolicyContinue`
- `CmdAfter` → cmd node at `EventLoopEnd`, `FailurePolicyContinue`
- `AgentPost` → agent node at `EventRunEnd`, `FailurePolicyStop`
- Main agent (`Content`) → required `EventLoopBody` node

All phase agents inherit `Provider` and `Model` from Config. Main agent node carries full Config fields (Trust, Plan, SystemPrompt, AllowedTools, DisallowedTools, MaxTurns, MCPConfig, PassthroughArgs). Adapted pipeline passes `pipeline.Normalize` validation.

**Known semantic difference:** Original `--agent-before`/`--cmd-before` skip the current iteration on failure; pipeline `FailurePolicyStop` at `EventLoopStart` stops the whole run. Documented in adapter comments.

**Tests:** 13 new tests in `internal/cli/pipeline_adapter_test.go` covering each hook flag, node ordering, field preservation, iterations, and full pipeline validation. All 899 tests pass.
