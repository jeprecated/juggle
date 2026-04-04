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

## Completion Summary

Added `--plan` flag as a shortcut for read-only plan mode. Changed files:
- `internal/cli/juggle.go`: Added `Plan bool` to `Config` struct, `plan` to `flags` struct, registered `--plan` cobra flag, set `Plan: flags.plan` in cfg, added mutual exclusivity check in `Run()`.
- `internal/cli/watch.go`: Updated `buildRunOptions` to check `cfg.Plan → PermissionPlan` before `cfg.Trust → PermissionBypass`.
- `internal/cli/juggle_test.go`: Added `TestRunLoop_PlanMode` and `TestRun_PlanAndTrustMutuallyExclusive`.
- `internal/cli/watch_test.go`: Added "plan mode" case to `TestBuildRunOptions`.
