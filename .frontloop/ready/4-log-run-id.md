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
