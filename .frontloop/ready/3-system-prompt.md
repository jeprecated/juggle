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
