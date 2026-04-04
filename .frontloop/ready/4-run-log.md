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
