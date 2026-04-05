---
title: Add RunID to JSONL log entries
priority: low
---

## Goal

Add a `run_id` field to both `iterationLogEntry` and `summaryLogEntry` so log entries can be correlated across resume boundaries. Currently when --resume appends to the same log file, entries from different runs are indistinguishable.

## Problem

`log.go:13-24` — neither log struct includes RunID.

## Acceptance Criteria

- Add `RunID string` field (json:"run_id") to iterationLogEntry and summaryLogEntry
- Pass RunID from Config when calling writeIterationLog and writeSummaryLog
- Existing log tests updated to verify run_id field is present
- parseLastIteration still works (it ignores unknown fields, just verify)

## Completion Summary

Added `RunID string` (json:"run_id") to `iterationLogEntry` and `summaryLogEntry` in `log.go`. Added `runID string` to `runStats` struct in `juggle.go` and wired `cfg.RunID` into all `runStats` initializations across `juggle.go`, `watch.go`, and `watch_glob.go`. Updated `TestWriteIterationLog_JSONFields` and `TestWriteSummaryLog_JSONFields` to assert `run_id` is present; added `TestWriteIterationLog_IncludesRunID`, `TestWriteSummaryLog_IncludesRunID`, and updated `TestRunLoop_LogsTokensAndExitCode` to require `run_id` in the iteration entry.
