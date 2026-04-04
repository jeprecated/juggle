---
title: Deduplicate consecutive tool markers in streamed output
priority: medium
---

## Goal

Reduce visual noise from repeated `[Tool: Read]` markers in agent output by suppressing consecutive duplicates.

## Acceptance Criteria

- Consecutive identical `[Tool: X]` markers collapsed to a single one
- Different tool names still print separately
- Text output between tool uses resets the dedup tracker
- Both `"assistant"/"tool_use"` and `"system"` event paths are covered

## Design Decisions

- Track `lastTool string` in `streamJSONOutput()`, skip if name matches
- Reset `lastTool` when text content is printed
- Keep tool markers entirely (don't remove them) -- just deduplicate

## Completion Summary

- Added `lastTool` dedup tracking in `streamJSONOutput()` in `internal/agent/provider/shared.go`
- Covers both `assistant/tool_use` and `system` event paths, shared dedup state between them
- Reset on text output so same tool after text still prints
- Created `internal/agent/provider/shared_test.go` with 5 tests
- All 221 tests pass
