---
title: Shell command hooks (--cmd-before, --cmd-after)
priority: high
---

## Goal

Add `--cmd-before CMD` and `--cmd-after CMD` flags that execute shell commands before and after each iteration. This is the foundation for users to wire up their own verification gates, notifications, and cleanup without juggle being opinionated about conventions.

These are SHELL COMMANDS, distinct from:
- `--before`/`--after` — separate agent sessions (multi-phase feature)
- `--hook` — agent-internal hooks (Claude Code hook system)

## Acceptance Criteria

- `--cmd-before CMD` executes a shell command before each iteration (inline or `@file` reference)
- `--cmd-after CMD` executes a shell command after each iteration (inline or `@file` reference)
- `@file` references resolve via JUGGLE_PROMPTS → cwd (same as prompt @files and --hook)
- Cmd-before failure (non-zero exit) skips the iteration and logs a warning
- Cmd-after failure logs a warning but doesn't stop the loop
- Both hooks receive iteration metadata as environment variables
- After-cmd also receives exit code and token counts from the iteration
- Works in both normal loop and watch mode
- Tests cover success, failure, and missing hook cases

## Implementation Notes

- Environment variables: `JUGGLE_ITERATION`, `JUGGLE_MAX_ITERATIONS`, `JUGGLE_EXIT_CODE`, `JUGGLE_INPUT_TOKENS`, `JUGGLE_OUTPUT_TOKENS`
- If value starts with `@`, resolve file path via ResolveArgs (JUGGLE_PROMPTS → cwd), then execute the resolved file
- Otherwise execute inline command via `exec.Command("sh", "-c", cmd)` for shell expansion
- Hook output goes to stderr
