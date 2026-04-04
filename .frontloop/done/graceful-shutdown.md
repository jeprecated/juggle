---
title: Graceful SIGINT/SIGTERM shutdown
priority: high
---

## Goal

Trap SIGINT/SIGTERM so the first signal finishes the current iteration cleanly, prints a run summary, and exits. A second signal force-kills immediately. Prevents orphaned agent processes and mid-write corruption.

## Acceptance Criteria

- First SIGINT/SIGTERM sets a "shutting down" flag
- Current in-flight iteration is allowed to complete (up to --timeout)
- No new iteration is started after the flag is set
- Run summary (iterations completed, tokens, duration) printed on clean exit
- Second signal force-kills the process immediately
- Agent child process is also terminated on force-kill
- Works in both RunLoop and RunWatch
- Exit code 130 for signal-interrupted runs
- Tests verify flag prevents next iteration from starting

## Implementation Notes

- Use signal.NotifyContext or a channel-based signal handler
- Check shutdown flag at the top of each iteration loop
- On force-kill, send SIGTERM to child process group

## Completion Summary

- Added `ErrInterrupted` sentinel error to `internal/cli/juggle.go`
- Added `Shutdown <-chan struct{}` and `ForceCtx context.Context` fields to `Config`
- Added `runStats` struct and `printRunSummary` for run summary output
- Updated `RunLoop` to check shutdown at top of each iteration; prints summary on interrupt
- Made rate-limit waits and inter-iteration delays interruptible via shutdown channel
- Added `*runStats` parameter to `runWatchTask` for cross-task stat accumulation
- Updated `RunWatch` outer loop to check shutdown and print summary; made poll sleep interruptible
- Added channel-based signal handler in `run()`: first signal closes shutdown channel, second cancels force context then calls `os.Exit(130)`
- Added `Context context.Context` to `provider.RunOptions` for external cancellation
- Updated `claude.go` and `opencode.go` to use `opts.Context` as base context (enables child kill on second signal)
- Updated `main.go` to detect `ErrInterrupted` and exit with code 130
- Added 6 new tests covering all shutdown scenarios

### Files Changed

- `internal/cli/juggle.go` (modified)
- `internal/cli/watch.go` (modified)
- `internal/cli/juggle_test.go` (modified)
- `internal/cli/watch_test.go` (modified)
- `internal/agent/provider/provider.go` (modified)
- `internal/agent/provider/claude.go` (modified)
- `internal/agent/provider/opencode.go` (modified)
- `cmd/juggle/main.go` (modified)
