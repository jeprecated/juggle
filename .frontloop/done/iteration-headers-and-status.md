---
title: Add iteration headers and status lines to loop output
priority: high
---

## Goal

Make juggle loop output readable by printing clear iteration separators and post-iteration status with timing and token usage.

## Acceptance Criteria

- Header printed to stderr before each iteration: `── Iteration 1/10 ──`
- Watch mode headers include task filename: `── Iteration 1/5 [task.md] ──`
- Status line printed to stderr after each iteration: `  12s | 1523 in / 892 out (1200 cached)`
- Dim gray styling via lipgloss when stderr is a TTY, plain text otherwise
- Existing tests pass, new tests cover formatter

## Design Decisions

- All chrome goes to stderr (consistent with existing delay/rate-limit messages, keeps stdout clean for piping)
- Single `LoopFormatter` struct in `internal/cli/format.go` with `IterationHeader()` and `IterationStatus()` methods
- TTY detection via `os.ModeCharDevice` check on the writer
- lipgloss dependency already added to go.mod

## Completion Summary

- Created `internal/cli/format.go` with `LoopFormatter`, `NewLoopFormatter(w)`, `IterationHeader()`, `IterationStatus()`
- Modified `RunLoop()` in `internal/cli/juggle.go`: formatter prints header before each iteration, status after success
- Modified `runWatchTask()` in `internal/cli/watch.go`: same pattern, passes filename to header
- Created `internal/cli/format_test.go` with 8 tests covering headers, status, watch mode, and non-TTY output
- All 201 existing tests pass
