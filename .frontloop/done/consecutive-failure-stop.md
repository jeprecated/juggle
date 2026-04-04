---
title: Consecutive failure stop
priority: high
---

## Goal

After N consecutive non-zero exit codes from the agent (default 3), stop the loop with a clear diagnostic instead of burning remaining iterations on a broken config or stuck agent.

## Acceptance Criteria

- `--max-failures N` flag (default 3) sets the consecutive failure threshold
- Counter resets to 0 on any successful (exit 0) iteration
- On threshold breach, exit with clear message: "stopping: N consecutive failures"
- Rate-limited iterations don't count as failures (they're retried)
- Works in both RunLoop and RunWatch
- `--max-failures 0` disables the check
- Tests verify counter reset on success and threshold breach

## Implementation Notes

- Simple counter variable in the loop, reset on success
- Check after result processing, before delay/fuzz sleep

## Completion Summary

- Added `MaxFailures int` field to `Config` struct in `juggle.go`
- Added `--max-failures` flag (default 3) bound to `flags.maxFailures` in `init()`
- Wired `MaxFailures` from flags into `Config` in the `run()` handler
- Added `consecutiveFailures` counter in `RunLoop`: increments on non-zero exit, resets on exit 0, stops with `"stopping: N consecutive failures"` when threshold reached
- Added same counter logic in `runWatchTask` in `watch.go`
- Rate-limited iterations continue to skip the failure-counting path (via `continue` before it)
- `MaxFailures=0` disables the check (guarded by `cfg.MaxFailures > 0`)

### Files Changed

- `internal/cli/juggle.go` (modified)
- `internal/cli/watch.go` (modified)
- `internal/cli/juggle_test.go` (modified)
- `internal/cli/watch_test.go` (modified)
