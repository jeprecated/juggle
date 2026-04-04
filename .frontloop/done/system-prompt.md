---
title: Expose system prompt flag (--system-prompt)
priority: medium
---

## Goal

Expose `--system-prompt` flag to set the agent's system prompt. Claude Code supports `--append-system-prompt` — juggle already has the `SystemPrompt` field on RunOptions but doesn't expose it as a CLI flag.

## Acceptance Criteria

- `--system-prompt "text"` sets the system prompt for all iterations
- `--system-prompt @file.md` resolves via @file (JUGGLE_PROMPTS → cwd)
- Passed to Claude Code as `--append-system-prompt`
- Mapped appropriately for other providers
- Works in both headless and interactive modes
- Tests verify system prompt is passed to agent

## Implementation Notes

- RunOptions.SystemPrompt already exists and claude.go already handles it (line ~70)
- Just need to add the cobra flag and wire it through Config → buildRunOptions
- Support @file resolution same as other flags

## Completion Summary

- Added `SystemPrompt string` field to `Config` struct
- Added `systemPrompt string` to the cobra `flags` struct
- Registered `--system-prompt` cobra flag with @file resolution hint in description
- Resolved `@file` references in `run()` using `ResolveArgs` before building Config
- Wired `SystemPrompt` through `Config` → `buildRunOptions()` → `RunOptions`
- Added `TestBuildRunOptions_SystemPrompt` in `watch_test.go`
- Added `TestRunLoop_SystemPrompt` in `juggle_test.go`

### Files Changed

- `internal/cli/juggle.go` (modified)
- `internal/cli/watch.go` (modified)
- `internal/cli/watch_test.go` (modified)
- `internal/cli/juggle_test.go` (modified)
