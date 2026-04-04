---
title: Pass-through flags to agent CLI (--)
priority: medium
---

## Goal

Support `--` separator to pass arbitrary flags directly to the underlying agent CLI. Everything after `--` is appended to the agent command as-is. Useful for provider-specific flags juggle doesn't know about.

## Acceptance Criteria

- `juggle @task.md -- --max-turns 50 --allowedTools Bash,Read` passes `--max-turns 50 --allowedTools Bash,Read` to the agent CLI
- Works with all providers (claude, opencode, custom)
- Pass-through flags appear after juggle's own flags in the spawned command
- `--` with nothing after it is a no-op
- Pass-through flags documented in help text with examples
- Tests verify flags are appended to agent command

## Completion Summary

- Added `PassthroughArgs []string` to `RunOptions` in `internal/agent/provider/provider.go`
- Appended `opts.PassthroughArgs` at end of args in `claudeHeadlessArgs` (`claude.go`) and both opencode run modes (`opencode.go`)
- Added `PassthroughArgs []string` to `Config` in `internal/cli/juggle.go`
- Wired through `buildRunOptions` in `internal/cli/watch.go`
- Added `splitPassthroughArgs` helper in `juggle.go` using `cmd.Flags().ArgsLenAtDash()`; wired into `run()` cobra handler
- Added `--` example to root command help text
- Tests: `TestClaudeHeadlessArgs_PassthroughArgs`, `TestClaudeHeadlessArgs_PassthroughArgs_Empty`, `TestBuildRunOptions_PassthroughArgs`, `TestRunLoop_PassthroughArgs`, `TestSplitPassthroughArgs`
