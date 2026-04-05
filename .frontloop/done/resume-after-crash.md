---
title: Resume after crash (--resume)
priority: low
---

## Goal

Add `--resume` flag that reads the log file to determine the last completed iteration and continues from there. Useful when an overnight run gets interrupted by a crash, reboot, or OOM kill.

## Acceptance Criteria

- `--resume` requires `--log` to be set (errors if not)
- Reads the log file, finds the highest completed iteration number
- Starts the loop from iteration N+1
- Logs "resuming from iteration N+1" to stderr
- If log file doesn't exist or is empty, starts from 1
- Works in normal loop mode (watch mode has its own resume via directory state)
- Tests verify resume from various log states

## Design Decisions

- Resume depends on --log being set since the log file is the source of truth for what completed

## Implementation Notes

- Parse JSONL log file, find max iteration number
- Adjust the loop start index in RunLoop

## Completion Summary

- Added `Resume bool` field to `Config` struct
- Added `--resume` flag registered in `init()` and wired through `run()` → `cfg`
- `RunLoop` validates `--resume` requires `--log` (returns error if not set)
- `RunLoop` calls `parseLastIteration` to find last completed iteration; starts loop from `last+1`
- Logs "resuming from iteration N" to stderr when last > 0
- `writeIterationLog` appends a JSONL entry `{"iteration":N}` per completed iteration after each successful iteration
- Added `internal/cli/log.go` with `writeIterationLog` and `parseLastIteration`
- Added `internal/cli/log_test.go` with full test coverage (8 tests)

### Files Changed

- `internal/cli/juggle.go` (modified)
- `internal/cli/log.go` (new)
- `internal/cli/log_test.go` (new)
