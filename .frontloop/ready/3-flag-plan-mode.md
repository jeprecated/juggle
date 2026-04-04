---
title: Add --plan flag shortcut
priority: medium
---

## Goal

Add `--plan` as a shortcut for `--permission-mode plan` (read-only mode). Currently juggle has `--trust` for bypass but no convenient flag for plan mode. Useful for dry-run-like iterations where the agent can only read and suggest.

## Acceptance Criteria

- `--plan` sets permission mode to PermissionPlan
- `--plan` and `--trust` are mutually exclusive (error if both set)
- Maps to Claude Code's `--permission-mode plan`
- Maps to OpenCode's `--agent plan`
- Tests verify permission mapping and mutual exclusivity

## Implementation Notes

- Add Plan bool to flags struct and Config
- In buildRunOptions, check for Plan → set PermissionPlan
- Validate --plan and --trust aren't both set in run()
