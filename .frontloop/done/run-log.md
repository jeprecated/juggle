---
title: Run log (--log FILE)
priority: low
---

## Goal

Add `--log FILE` flag that appends a one-line JSON entry per iteration to a file. Enables post-mortem review of overnight runs — what happened, when, how many tokens, exit codes.

## Acceptance Criteria

- Each iteration appends one JSON line to the log file
- Fields: timestamp, iteration number, duration, input/output/cache tokens, exit code, rate_limited (bool), error (string or null)
- File is created if it doesn't exist, appended if it does
- Final summary line appended at end of run
- Log writes don't block or fail the loop (best-effort)
- Tests verify JSON line format and append behavior

## Implementation Notes

- JSONL format (one JSON object per line) for easy parsing with jq
- Use os.OpenFile with O_APPEND|O_CREATE|O_WRONLY

## Completion Summary

- Expanded `iterationLogEntry` in `log.go` with all required fields: `timestamp`, `duration_ms`, `input_tokens`, `output_tokens`, `cache_tokens`, `exit_code`, `rate_limited`, `error` (`*string`, null when none)
- Changed `writeIterationLog` signature to accept an `iterationLogEntry` value (built at call site in `RunLoop`)
- Added `summaryLogEntry` struct and `writeSummaryLog` function that appends a `{"type":"summary",...}` JSON line
- Updated `writeSummary` in `juggle.go` to call `writeSummaryLog` instead of writing plain text to the log
- Updated `--log` flag description to reflect new JSONL-per-iteration behavior
- Updated existing test `TestRunLoop_LogFileWritesSummary` to expect JSON summary line
- Added new tests: `TestWriteIterationLog_JSONFields`, `TestWriteIterationLog_NullError`, `TestWriteIterationLog_AppendsBehavior`, `TestWriteSummaryLog_JSONFields`, `TestRunLoop_LogsTokensAndExitCode`, `TestRunLoop_LogsSummaryLine`

### Files Changed

- `internal/cli/log.go` (modified)
- `internal/cli/log_test.go` (modified)
- `internal/cli/juggle.go` (modified)
- `internal/cli/juggle_test.go` (modified)
