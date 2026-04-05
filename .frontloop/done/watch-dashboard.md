---
title: Watch mode dashboard overview
priority: high
---

## Goal

Add a TUI dashboard that shows an overview of running watch workers, their current task, and iteration stats. Available in any watch mode (single dir or glob), activated by default when glob watch would make raw output unreadable. Replaces interleaved agent output with a scannable status screen.

## Acceptance Criteria

- Dashboard shows: repo/watch dir, worker status (active/idle), current task name, iteration progress
- Available in single-directory watch mode (opt-in via flag)
- Default behavior for glob watch patterns
- Updates in real time as iterations complete and tasks change
- Provides a way to drill into or tail a specific worker's output if needed

## Completion Summary

- Added `internal/cli/dashboard.go`: `Dashboard`, `WorkerState`, `WorkerStatus` types; `NewDashboard`, `Update`, `WorkerState`, `Render`, `Run` methods; `workerDashboard` helper with `setupWorkerDashboard`, `openWorkerLog`, `stop`
- Added `internal/cli/dashboard_test.go`: 12 TDD tests covering construction, rendering, updates, iteration progress, log paths, multi-worker independence
- Added `Dashboard bool` and `OnIterDone func(iter, maxIter int)` to `Config`
- Added `--dashboard` flag (Watch Mode group) to CLI in `juggle.go`
- Wired dashboard into `RunWatch` (single-dir serial, opt-in), `runWatchWorkers` (multi-worker), `runGlobWatchSerial` (auto-enabled), `runGlobWatchWorkers` (auto-enabled)
- Each worker gets a temp log file for output; dashboard shows log path for `tail -f` drill-down
- Added `TestRunWatchTask_OnIterDoneCalledPerIteration` to `watch_test.go`

### Files Changed

- `internal/cli/dashboard.go` (new)
- `internal/cli/dashboard_test.go` (new)
- `internal/cli/juggle.go` (modified)
- `internal/cli/watch.go` (modified)
- `internal/cli/watch_glob.go` (modified)
