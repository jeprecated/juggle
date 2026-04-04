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
