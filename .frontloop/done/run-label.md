---
title: Run label (--label)
priority: low
---

## Goal

Add `--label "description"` flag that tags the run with a human-readable name. Appears in iteration headers, log output, and any notification hooks. If omitted, auto-generate from first ~50 chars of prompt.

## Acceptance Criteria

- `--label "refactor auth"` sets the label for the run
- Label appears in iteration header output
- Label included in JSONL log entries if `--log` is set
- Label passed as `JUGGLE_LABEL` environment variable to hooks and agents
- Auto-generated from prompt content when not provided
- Tests verify label in headers and env vars

## Completion Summary

- Added `label` param to `IterationHeader` — shows `· label` after iteration info
- Added `Label string` field to `iterationLogEntry` with `omitempty` JSON tag
- Updated `writeIterationLog` to accept and write label to JSONL entries
- Added `autoLabel` function — trims and truncates content to 50 chars
- Updated `RunLoop` to auto-generate label from prompt when `cfg.Label` is empty
- Updated all `IterationHeader` and `writeIterationLog` call sites in juggle.go and watch.go
- Added tests: label in header, label omitted when empty, label in log, auto-label in header

### Files Changed

- `internal/cli/format.go` (modified)
- `internal/cli/log.go` (modified)
- `internal/cli/juggle.go` (modified)
- `internal/cli/watch.go` (modified)
- `internal/cli/format_test.go` (modified)
- `internal/cli/log_test.go` (modified)
