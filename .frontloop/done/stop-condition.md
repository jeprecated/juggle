---
title: Stop condition (--stop-when)
priority: high
---

## Goal

Add `--stop-when CMD` flag that runs a shell command after each iteration. If the command exits 0, the loop stops. This lets users define their own completion criteria (e.g., "all tests pass", "no more TODOs in progress file", "sentinel file exists").

## Acceptance Criteria

- `--stop-when CMD` executes after each iteration (and after --cmd-after hook if both set)
- Exit 0 = stop looping gracefully (not an error)
- Non-zero exit = continue looping
- Stop reason logged to stderr
- Works in both normal loop and watch mode
- Combines correctly with `--iterations` (whichever triggers first wins)
- Tests cover stop-triggered, continue, and interaction with max iterations

## Implementation Notes

- Receives same environment variables as --cmd-after hook
- Evaluated after --cmd-after but before delay/fuzz sleep

## Completion Summary

- Added `StopWhen string` field to `Config` struct
- Added `--stop-when` flag in `init()` and wired through `run()` → `cfg`
- In `RunLoop`: after cmd-after, runs stop-when hook; exit 0 logs "stop-when condition met" and returns nil
- In `runWatchTask`: same stop-when logic added after cmd-after block
- Tests: `TestRunLoop_StopWhenExitsZeroStops`, `TestRunLoop_StopWhenNonZeroContinues`, `TestRunLoop_StopWhenIterationsWinsFirst`, `TestRunWatchTask_StopWhenExitsZeroStops`

### Files Changed

- `internal/cli/juggle.go` (modified)
- `internal/cli/watch.go` (modified)
- `internal/cli/juggle_test.go` (modified)
- `internal/cli/watch_test.go` (modified)
