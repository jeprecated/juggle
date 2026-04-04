---
title: Expose --max-turns flag
priority: medium
---

## Goal

Add `--max-turns N` flag to cap the number of tool-use turns per iteration. Prevents a single iteration from consuming the entire context window and budget. Maps to Claude Code's `--max-turns`.

## Acceptance Criteria

- `--max-turns 50` limits tool-use turns per iteration
- Passed to Claude Code as `--max-turns N`
- Mapped to equivalent flag for other providers (or silently ignored with verbose warning)
- Default: unset (provider's own default applies)
- Tests verify flag is passed to agent command

## Implementation Notes

- Add MaxTurns int to RunOptions
- Add cobra flag, wire through Config → buildRunOptions → provider Run()
- Claude: append `--max-turns N` to args
- OpenCode: research equivalent flag

## Completion Summary

- Added `MaxTurns int` to `RunOptions` in `provider.go`
- Added `--max-turns N` to `claudeHeadlessArgs` when `MaxTurns > 0`
- Added verbose warning in opencode `runHeadless` when `MaxTurns > 0` (no equivalent flag)
- Added `MaxTurns int` to `Config` and `flags` in `juggle.go`
- Registered `--max-turns` cobra flag (default 0 = provider default)
- Wired `MaxTurns` through `run()` → `Config` → `buildRunOptions()` → `RunOptions`

### Files Changed

- `internal/agent/provider/provider.go` (modified)
- `internal/agent/provider/claude.go` (modified)
- `internal/agent/provider/opencode.go` (modified)
- `internal/cli/juggle.go` (modified)
- `internal/cli/watch.go` (modified)
- `internal/agent/provider/provider_test.go` (modified)
- `internal/cli/juggle_test.go` (modified)
