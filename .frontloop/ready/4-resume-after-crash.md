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
